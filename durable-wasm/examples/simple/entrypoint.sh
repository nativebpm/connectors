#!/bin/sh
set -e

DB_PATH="/app/snapshots.db"
CONFIG_PATH="/etc/litestream.yml"

# 1. Attempt to restore the SQLite database from S3 (rclone) if it doesn't exist locally
if [ ! -f "$DB_PATH" ]; then
    echo "[ENTRYPOINT] Local database not found. Attempting to restore from S3..."
    # We use -if-replica-exists so it doesn't fail on the very first run when the bucket is empty
    litestream restore -config "$CONFIG_PATH" -if-replica-exists "$DB_PATH"
fi

# 2. Run the host application wrapped in litestream replicate.
# This replicates WAL frames to S3 in real-time and gracefully exits when the host finishes.
echo "[ENTRYPOINT] Starting application with Litestream replication..."
exec litestream replicate -config "$CONFIG_PATH" -exec "/app/host"
