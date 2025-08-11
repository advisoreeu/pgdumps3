# pgdumps3

`pgdumps3` is a simple and containerized Go application that performs scheduled backups of a PostgreSQL database and uploads them to an S3-compatible object storage service. It can also restore a backup from S3 and exit if configured accordingly.

**Docker Image:**
[`ghcr.io/advisoreeu/pgdumps3:latest`](https://github.com/advisoreeu/pgdumps3/pkgs/container/pgdumps3)

## Features

* **Scheduled Backups:** Uses a cron expression to schedule regular backups.
* **PostgreSQL Version Detection:** Automatically detects the PostgreSQL version to use the correct `pg_dump` utility.
* **S3-Compatible:** Works with AWS S3 and other S3-compatible services like MinIO.
* **Restore Mode:** If `RESTORE_KEY` is set, the container will restore the database from the specified backup (key in S3) and exit.
* **Containerized:** Ready to be deployed as a Docker container.
* **Cross-Architecture Tested:** Verified on both `arm64` and `amd64` architectures.
* **PostgreSQL Compatibility:** Tested with PostgreSQL versions **15**, **16**, and **17**.

## Configuration

The application is configured using environment variables:

| Environment Variable   | Description                                                                                                       | Default Value | Required |
| ---------------------- | ----------------------------------------------------------------------------------------------------------------- | ------------- | -------- |
| `TIME_ZONE`            | The timezone to use for timestamps in backup filenames.                                                           | `UTC`         | No       |
| `S3_REGION`            | The region of the S3 service.                                                                                     | `us-east-1`   | No       |
| `S3_PATH_PREFIX`       | Prefix for backup filenames in the S3 bucket.                                                                     | `backups`     | No       |
| `S3_ENDPOINT`          | Endpoint of the S3-compatible service.                                                                            | —             | No       |
| `S3_ACCESS_KEY_ID`     | S3 access key ID.                                                                                                 | —             | **Yes**  |
| `S3_SECRET_ACCESS_KEY` | S3 secret access key.                                                                                             | —             | **Yes**  |
| `S3_BUCKET`            | S3 bucket name for storing backups.                                                                               | —             | **Yes**  |
| `S3_SSL`               | Use SSL when connecting to S3 (`true` or `false`).                                                                | `false`       | No       |
| `DB_USER`              | Username for the PostgreSQL server.                                                                               | —             | **Yes**  |
| `DB_PASSWORD`          | Password for the PostgreSQL server.                                                                               | —             | **Yes**  |
| `DB_NAME`              | Name of the database to back up or restore.                                                                       | —             | **Yes**  |
| `DB_HOST`              | Hostname or IP address of the PostgreSQL server.                                                                  | —             | **Yes**  |
| `DB_PORT`              | PostgreSQL server port.                                                                                           | `5432`        | No       |
| `LOG_LEVEL`            | Logging level: `debug`, `info`, `warn`, or `error`.                                                               | `info`        | No       |
| `CRON_SCHEDULE`        | Cron expression defining the backup schedule.                                                                     | `@daily`      | No       |
| `RESTORE_KEY`          | S3 object key to restore from **(does not include the bucket name)**. If set, the app will restore and then exit. | —             | No       |
| `DUMP_INFIX`           | String inserted in the middle of the backup filename.                                                             | —             | No       |
| `DUMP_SUFFIX`          | File suffix for backup files.                                                                                     | `.sql.gz`     | No       |

### Restore Mode

If `RESTORE_KEY` is set, `pgdumps3` will:

1. Download the specified backup from S3 (the value should be the object key, excluding the bucket name).
2. Restore it into the configured PostgreSQL database.
3. Exit immediately without running scheduled backups.

### Cron Schedule Examples

| Cron Expression | Description                                     |
| --------------- | ----------------------------------------------- |
| `@hourly`       | Run at the beginning of every hour.             |
| `@daily`        | Run once a day, at midnight.                    |
| `@weekly`       | Run once a week, at midnight on Sunday morning. |
| `@monthly`      | Run once a month, at midnight on the first day. |
| `0 3 * * *`     | Run at 3:00 AM every day.                       |
| `0 0 * * 0`     | Run at midnight every Sunday.                   |
| `*/10 * * * *`  | Run every 10 minutes.                           |
[Reference](https://pkg.go.dev/github.com/pocketbase/pocketbase/tools/cron#NewSchedule)

---

## Example Setup with Docker Compose

Below is an example `docker-compose.yml` that runs PostgreSQL and `pgdumps3` together:

```yaml
version: "3.8"

services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: postgres
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "127.0.0.1:5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  pgdumps3:
    image: ghcr.io/advisoreeu/pgdumps3:latest
    env_file:
      - example.env
    depends_on:
      - postgres

volumes:
  postgres_data:
```

---

## Example `.env` file

Save this as `.env` in the same directory as `docker-compose.yml`:

```env
# PostgreSQL
DB_HOST=postgres
DB_PORT=5432
DB_NAME=mypassword
DB_USER=postgres
DB_PASSWORD=myuser

# S3 Storage
S3_ENDPOINT=http://minio:9000
S3_ACCESS_KEY_ID=minioadmin
S3_SECRET_ACCESS_KEY=minioadmin
S3_BUCKET=mybucket
S3_REGION=us-east-1
S3_PATH_PREFIX=backups
S3_SSL=true

# App Settings
TIME_ZONE=UTC
LOG_LEVEL=info
CRON_SCHEDULE=@daily
DUMP_INFIX=
DUMP_SUFFIX=.sql.gz
RESTORE_KEY=

# Example: to restore from S3, set RESTORE_KEY and start the container
# RESTORE_KEY=backups/mydatabase-2025-08-10.sql.gz
```

---

## Development

To start the local development environment (includes PostgreSQL, MinIO, and `pgdumps3`):

```bash
docker-compose -f docker-compose.dev.yml up
```

MinIO console: [http://localhost:9001](http://localhost:9001)
Credentials: `minioadmin` / `minioadmin`

---

## Testing

We run **end-to-end (E2E) tests weekly** that verify:

1. A backup can be created from a running PostgreSQL instance.
2. The backup can be restored into a fresh PostgreSQL instance without errors.

These tests are executed on both **arm64** and **amd64** architectures for PostgreSQL **15**, **16**, and **17**.
