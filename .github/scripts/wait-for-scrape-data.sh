#!/usr/bin/env bash
# Wait for Thanos/Prometheus to have scrape data, then wait for enough
# history to satisfy range queries with since=3m.
#
# Usage: .github/scripts/wait-for-scrape-data.sh [thanos-url]
#   thanos-url  Thanos query endpoint (default: http://localhost:9090)

set -euo pipefail

THANOS_URL="${1:-http://localhost:9090}"
MAX_ATTEMPTS=60
POLL_INTERVAL=5
HISTORY_WAIT=180

echo "Waiting for scrape data at $THANOS_URL ..."

for i in $(seq 1 "$MAX_ATTEMPTS"); do
  count=$(curl -sf "${THANOS_URL}/api/v1/query?query=up" \
    | jq '.data.result | length' 2>/dev/null || echo 0)
  if [ "$count" -gt 0 ]; then
    echo "Thanos has data ($count series)"
    break
  fi
  echo "  attempt $i/$MAX_ATTEMPTS: waiting for scrape data..."
  sleep "$POLL_INTERVAL"
done

if [ "$count" -eq 0 ] 2>/dev/null || [ -z "$count" ]; then
  echo "FAIL: no metric data after $((MAX_ATTEMPTS * POLL_INTERVAL)) seconds"
  exit 1
fi

echo "Waiting ${HISTORY_WAIT}s for range query coverage (since: 3m)..."
sleep "$HISTORY_WAIT"
echo "Scrape history ready"
