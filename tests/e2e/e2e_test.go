package e2e

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/build"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	minioTc "github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

var tests = []struct {
	name string
}{
	{"postgres:15-alpine"},
	{"postgres:16-alpine"},
	{"postgres:17-alpine"},
}

func TestMain(t *testing.T) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testE2E(t, tt.name)
		})
	}
}

func testE2E(t *testing.T, postgresImage string) {
	t.Helper()

	ctx := context.Background()

	newNetwork, err := network.New(ctx)
	require.NoError(t, err)

	testcontainers.CleanupNetwork(t, newNetwork)
	networkName := newNetwork.Name

	postgresSourceName := "postgres-source"
	postgresDestName := "postgres-dest"
	minioName := "minio"
	minioContainer, err := minioTc.Run(
		ctx,
		"minio/minio:RELEASE.2024-01-16T16-07-38Z",
		network.WithNetworkName([]string{minioName}, networkName),
	)

	require.NoError(t, err)

	testcontainers.CleanupContainer(t, minioContainer)
	endpoint, err := minioContainer.Endpoint(ctx, "")

	require.NoError(t, err)

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})

	require.NoError(t, err)
	// Make a new bucket called testbucket.
	bucketName := "test-bucket"
	location := "us-east-1"

	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location})
	require.NoError(t, err)

	const dbName = "postgres"

	postgresSourceContainer, err := postgres.Run(ctx,
		postgresImage,
		postgres.WithInitScripts(filepath.Join("..", "postgres-initdb.d", "pgbench.sh")),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbName),
		postgres.WithPassword(dbName),
		postgres.BasicWaitStrategies(),
		network.WithNetworkName([]string{postgresSourceName}, networkName),
	)
	testcontainers.CleanupContainer(t, postgresSourceContainer)
	require.NoError(t, err)

	dumper := testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:   "../..",
				KeepImage: true,
				BuildOptionsModifier: func(ibo *build.ImageBuildOptions) {
					ibo.Version = build.BuilderBuildKit
				},
			},
			Env: map[string]string{
				"DB_HOST":              postgresSourceName,
				"DB_PORT":              "5432",
				"DB_NAME":              dbName,
				"DB_USER":              dbName,
				"DB_PASSWORD":          dbName,
				"S3_ENDPOINT":          fmt.Sprintf("http://%s:9000", minioName),
				"S3_ACCESS_KEY_ID":     "minioadmin",
				"S3_SECRET_ACCESS_KEY": "minioadmin",
				"S3_REGION":            "us-east-1",
				"S3_BUCKET":            "test-bucket",
				"S3_PATH_PREFIX":       "backups",
				"S3_SSL":               "false",
				"CRON_SCHEDULE":        "* * * * *",
				"DUMP_INFIX":           "_dev",
				"LOG_LEVEL":            "debug",
			},
			WaitingFor: &wait.LogStrategy{
				Occurrence:   1,
				Log:          "Successfully uploaded backup to S3",
				PollInterval: 1 * time.Second,
			},
			Networks: []string{networkName},
		},
	}

	dumperContainer, err := testcontainers.GenericContainer(ctx, dumper)

	testcontainers.CleanupContainer(t, dumperContainer)
	require.NoError(t, err)

	r, err := dumperContainer.Logs(ctx)
	require.NoError(t, err)

	key, err := ExtractBackupFilename(r, "minio:9000")
	require.NoError(t, err)

	postgresDestContainer, err := postgres.Run(ctx,
		postgresImage,
		postgres.WithInitScripts(filepath.Join("..", "postgres-initdb.d", "pgbench.sh")),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbName),
		postgres.WithPassword(dbName),
		postgres.BasicWaitStrategies(),
		network.WithNetworkName([]string{postgresDestName}, networkName),
	)
	testcontainers.CleanupContainer(t, postgresDestContainer)
	require.NoError(t, err)

	restore := testcontainers.GenericContainerRequest{
		Started: true,
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:   "../..",
				KeepImage: true,
				BuildOptionsModifier: func(ibo *build.ImageBuildOptions) {
					ibo.Version = build.BuilderBuildKit
				},
			},
			Env: map[string]string{
				"DB_HOST":              postgresDestName,
				"DB_PORT":              "5432",
				"DB_NAME":              dbName,
				"DB_USER":              dbName,
				"DB_PASSWORD":          dbName,
				"S3_ENDPOINT":          fmt.Sprintf("http://%s:9000", minioName),
				"S3_ACCESS_KEY_ID":     "minioadmin",
				"S3_SECRET_ACCESS_KEY": "minioadmin",
				"S3_REGION":            "us-east-1",
				"S3_BUCKET":            "test-bucket",
				"S3_PATH_PREFIX":       "backups",
				"S3_SSL":               "false",
				"CRON_SCHEDULE":        "* * * * *",
				"DUMP_INFIX":           "_dev",
				"LOG_LEVEL":            "debug",
				"RESTORE_KEY":          key,
			},
			WaitingFor: &wait.LogStrategy{
				Occurrence:   1,
				Log:          "successfully restored database",
				PollInterval: 1 * time.Second,
			},
			Networks: []string{networkName},
		},
	}

	restoreContainer, err := testcontainers.GenericContainer(ctx, restore)

	testcontainers.CleanupContainer(t, restoreContainer)
	require.NoError(t, err)

	r, err = restoreContainer.Logs(ctx)
	require.NoError(t, err)

	logs, err := io.ReadAll(r)
	require.NoError(t, err)

	fmt.Println(string(logs))
}
