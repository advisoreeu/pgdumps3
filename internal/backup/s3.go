package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	s3UploadPartSizeMB = 10
	mb                 = 1024 * 1024
)

// NewS3 creates a new S3 session and service client from the configuration.
func NewS3(config *Config) (*session.Session, error) {
	awsConfig := &aws.Config{
		Region: aws.String(config.S3Region),
		Credentials: credentials.NewStaticCredentials(
			config.S3AccessID,
			config.S3SecretKey,
			"",
		),
		S3ForcePathStyle: aws.Bool(true),
	}

	if config.S3Endpoint != "" {
		awsConfig.Endpoint = aws.String(config.S3Endpoint)
	}

	if !config.S3SSL {
		awsConfig.DisableSSL = aws.Bool(true)
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 session: %w", err)
	}

	return sess, nil
}

// PgDumpToS3 performs a pg_dump and uploads the output to an S3 bucket.
func PgDumpToS3(
	ctx context.Context,
	sess *session.Session,
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

	uploader := s3manager.NewUploader(sess, func(u *s3manager.Uploader) {
		u.PartSize = s3UploadPartSizeMB * mb
		u.Concurrency = 5
	})

	result, err := uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(config.S3Bucket),
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		if cmd.Process != nil {
			err = cmd.Process.Kill()
			if err != nil {
				slog.Error("failed to kill pg_dump process", "error", err)
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
