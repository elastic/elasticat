#!/bin/sh
set -euo pipefail

# Always run from this script's directory
cd "$(dirname "$0")"

# Ensure no stale containers are left around
docker compose down --remove-orphans

# Rebuild everything to avoid stale images
docker compose build

# Bring the stack back up
docker compose up -d

# Prepare log output
mkdir -p logs
LOG_FILE="logs/demo.log"

# Stream app logs to a file for elasticat to consume
docker compose logs -f gateway stock-service portfolio-service frontend 2>&1 > "$LOG_FILE" &
LOGS_PID=$!
trap "kill $LOGS_PID" EXIT

# Start elasticat watcher
../../bin/elasticat watch "$LOG_FILE"
