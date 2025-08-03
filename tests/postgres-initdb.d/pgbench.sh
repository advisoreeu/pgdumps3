#!/usr/bin/env bash
set -e

# Fix locale warning
export LC_ALL=C

# Config
SCALE=${SCALE:-10}
DB=${DB_NAME:-postgres}
USER=${POSTGRES_USER:-postgres}

echo "=== High-Entropy PostgreSQL Test Data ==="
echo "Database: $DB, User: $USER, Scale: $SCALE"

# Initialize pgbench
pgbench -iq -s $SCALE --unlogged -I dtg --fillfactor 10 $DB -U $USER

