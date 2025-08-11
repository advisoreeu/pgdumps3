package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	s3TransferPartSizeMB = 10
	mb                   = 1024 * 1024
)

// NewS3 creates a new S3 session and service client from the configuration.
func NewS3(ctx context.Context, config *Config) (*s3.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(config.S3Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(config.S3AccessID, config.S3SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if config.S3Endpoint != "" {
			o.BaseEndpoint = aws.String(config.S3Endpoint)
		}
		o.UsePathStyle = true
	}), nil
}

// PgDumpToS3 performs a pg_dump and uploads the output to an S3 bucket.
func PgDumpToS3(
	ctx context.Context,
	s3Client *s3.Client,
	pg *Postgres,
	config *Config,
) error {
	key := generateDumpName(config, pg.MajorVersion)
	slog.Info("Starting pg_dump to S3", "bucket", config.S3Bucket, "key", key)

	// gosec:G204
	cmd := exec.CommandContext(ctx, pg.PGDumpPath,
		"-h", config.DBHost,
		"-p", strconv.Itoa(config.DBPort),
		"-U", config.DBUser,
		"-d", config.DBName,
		"--no-password",
		"--verbose",
		"--clean",
		"--if-exists",
		"--create",
		"-Z", "6",
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", config.DBPassword))

	reader, writer := io.Pipe()
	cmd.Stdout = writer

	var stderrBuf bytes.Buffer

	if config.LogLevel == "debug" {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &stderrBuf
	}

	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if err := writer.Close(); err != nil {
				slog.Error("failed to close writer", "error", err)
			}
		}()

		if err := cmd.Run(); err != nil {
			slog.Error("pg_dump failed", "error", err, "stderr", stderrBuf.String())

			errChan <- err

			return
		}

		errChan <- nil
	}()

	uploader := manager.NewUploader(s3Client, func(u *manager.Uploader) {
		u.PartSize = s3TransferPartSizeMB * mb
		u.Concurrency = 5
	})

	result, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(config.S3Bucket),
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		if cmd.Process != nil {
			killErr := cmd.Process.Kill()
			if killErr != nil {
				slog.Error("failed to kill pg_dump process", "error", killErr)
			}
		}

		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	if err := <-errChan; err != nil {
		return fmt.Errorf("pg_dump command failed: %w", err)
	}

	slog.Info("Successfully uploaded backup to S3", "location", result.Location)

	return nil
}

// generateDumpName creates a backup filename based on the configuration.
func generateDumpName(config *Config, pgVersion int) string {
	now := time.Now().In(config.TimeZone)
	date := now.Format("2006-01-02T15-04-05")
	filename := fmt.Sprintf(
		"pg%d_%s_%s%s%s",
		pgVersion,
		config.DBName,
		date,
		config.DumpInfix,
		config.DumpSuffix,
	)

	return path.Join(config.S3PathPrefix, filename)
}

func Restore(
	ctx context.Context,
	s3Client *s3.Client,
	pg *Postgres,
	config *Config,
	key string,
) (err error) {
	slog.Info("starting pg_restore", "db", config.DBName, "bucket_key", key)

	getObject := s3.GetObjectInput{Bucket: &config.S3Bucket, Key: &key}

	result, getErr := s3Client.GetObject(ctx, &getObject, func(so *s3.Options) {})
	if getErr != nil {
		return getErr
	}

	defer func() {
		if cerr := result.Body.Close(); cerr != nil {
			if err == nil {
				err = cerr
			} else {
				err = fmt.Errorf("%w; close S3 body: %v", err, cerr)
			}
		}
	}()

	gzr, gzErr := gzip.NewReader(result.Body)
	if gzErr != nil {
		return fmt.Errorf("failed to create gzip reader: %w", gzErr)
	}

	defer func() {
		if cerr := gzr.Close(); cerr != nil {
			if err == nil {
				err = cerr
			} else {
				err = fmt.Errorf("%w; close gzip reader: %v", err, cerr)
			}
		}
	}()

	cmd := exec.CommandContext(ctx, pg.psqlPath,
		"-h", config.DBHost,
		"-p", strconv.Itoa(config.DBPort),
		"-U", config.DBUser,
		"-d", "template1", // use safe DB for --create dumps
		"--no-password",
	)
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("PGPASSWORD=%s", config.DBPassword))
	cmd.Stdin = gzr

	var stderrBuf bytes.Buffer

	if config.LogLevel == "debug" {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &stderrBuf
	}

	if runErr := cmd.Run(); runErr != nil {
		if err == nil {
			err = fmt.Errorf("psql restore failed: %w", runErr)
		} else {
			err = fmt.Errorf("%w; psql restore failed: %v", err, runErr)
		}

		return err
	}

	slog.Info("successfully restored database", "db", config.DBName)

	return err
}
