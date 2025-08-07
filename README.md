# pgdumps3

`pgdumps3` is a simple and containerized Go application that performs scheduled backups of a PostgreSQL database and uploads them to an S3-compatible object storage service.

## Features

*   **Scheduled Backups:** Uses a cron expression to schedule regular backups.
*   **PostgreSQL Version Detection:** Automatically detects the PostgreSQL version to use the correct `pg_dump` utility.
*   **S3-Compatible:** Works with AWS S3 and other S3-compatible services like MinIO.
*   **Customizable:** Highly configurable through environment variables.
*   **Containerized:** Ready to be deployed as a Docker container.

## Configuration

The application is configured using environment variables. The following variables are available:

| Environment Variable      | Description                                                                 | Default Value     |
| ------------------------- | --------------------------------------------------------------------------- |
| `DB_HOST`                 | The hostname or IP address of the PostgreSQL server.                        | `postgres`        |
| `DB_PORT`                 | The port of the PostgreSQL server.                                          | `5432`            |
| `DB_NAME`                 | The name of the database to back up.                                        |                   |
| `DB_USER`                 | The username to connect to the PostgreSQL server.                           |                   |
| `DB_PASSWORD`             | The password to connect to the PostgreSQL server.                           |                   |
| `S3_ENDPOINT`             | The endpoint of the S3-compatible service.                                  |                   |
| `S3_ACCESS_KEY_ID`        | The access key ID for the S3 service.                                       |                   |
| `S3_SECRET_ACCESS_KEY`    | The secret access key for the S3 service.                                   |                   |
| `S3_REGION`               | The region of the S3 service.                                               | `us-east-1`       |
| `S3_BUCKET`               | The name of the S3 bucket to upload the backups to.                         |                   |
| `S3_PATH_PREFIX`          | A prefix to add to the backup filenames in the S3 bucket.                   | `backups`         |
| `S3_SSL`                  | Whether to use SSL to connect to the S3 service.                            | `false`           |
| `CRON_SCHEDULE`           | A cron expression that defines the backup schedule.                         | `@daily`          |
| `DUMP_INFIX`              | A string to insert in the middle of the backup filename.                    |                   |
| `DUMP_SUFFIX`             | The suffix to add to the backup filename.                                   | `.sql.gz`         |
| `LOG_LEVEL`               | The log level. Can be `debug`, `info`, `warn`, or `error`.                  | `info`            |
| `TIME_ZONE`               | The timezone to use for the timestamps in the backup filenames.             | `UTC`             |

### Cron Schedule Examples

The `CRON_SCHEDULE` environment variable uses the standard cron format. Here are some examples:

| Cron Expression     | Description                                       |
| ------------------- | ------------------------------------------------- |
| `@hourly`           | Run at the beginning of every hour.               |
| `@daily`            | Run once a day, at midnight.                      |
| `@weekly`           | Run once a week, at midnight on Sunday morning.   |
| `@monthly`          | Run once a month, at midnight on the first day of the month. |
| `0 3 * * *`         | Run at 3:00 AM every day.                         |
| `0 0 * * 0`         | Run at midnight on every Sunday.                  |
| `*/10 * * * *`      | Run every 10 minutes.                             |

## Development

The project includes a `docker-compose.dev.yml` file for setting up a local development environment. This will start three services:

*   `postgres`: A PostgreSQL 16 database.
*   `minio`: A MinIO S3-compatible object storage service.
*   `app`: The `pgdumps3` application.

To start the development environment, run:

```bash
docker-compose -f docker-compose.dev.yml up
```

The application will be built and run with `air` for live reloading. You can access the MinIO console at `http://localhost:9001` with the credentials `minioadmin`/`minioadmin`.
