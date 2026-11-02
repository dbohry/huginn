#!/bin/sh
# Runs every browser check against bin/huginn, each on its own empty data directory.
# Build first with `make build`. Needs Chromium: run `npm install` in this directory once.
set -eu
cd "$(dirname "$0")/.."
port=18100
for check in flows a11y; do
  data=$(mktemp -d)
  HUGINN_DATA_DIR="$data" HUGINN_ADDR="127.0.0.1:$port" bin/huginn serve >"$data/server.log" 2>&1 &
  pid=$!
  sleep 1
  status=0
  BASE="http://127.0.0.1:$port" node "e2e/$check.js" || status=$?
  kill "$pid"
  [ "$status" -eq 0 ] || { echo "--- server log ---"; cat "$data/server.log"; exit "$status"; }
  port=$((port + 1))
done
