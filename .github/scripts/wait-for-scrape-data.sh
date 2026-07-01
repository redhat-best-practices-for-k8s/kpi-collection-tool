#!/usr/bin/env bash
# Wait for Thanos/Prometheus to have scrape data, then wait for enough
# history to satisfy range queries with since=3m.
#
# Usage: .github/scripts/wait-for-scrape-data.sh <thanos-url> [metric-query...]
#   thanos-url    Thanos query endpoint (default: http://localhost:9090)
#   metric-query  Additional PromQL queries that must return data before the
#                 history wait begins (e.g. "kube_pod_info" or "up{job='foo'}").
#                 This prevents starting the timer before slower exporters
#                 (like kube-state-metrics) are scraped.
#
# Environment variables (optional, for authenticated endpoints):
#   BEARER_TOKEN   - Bearer token for Authorization header
#   INSECURE_TLS   - Set to "true" to skip TLS verification (-k)

set -euo pipefail

THANOS_URL="${1:-http://localhost:9090}"
shift || true
EXTRA_METRICS=("$@")

MAX_ATTEMPTS=80
POLL_INTERVAL=5
HISTORY_WAIT=210

CURL_OPTS=(-sf)
if [ "${INSECURE_TLS:-}" = "true" ]; then
  CURL_OPTS+=(-k)
fi
if [ -n "${BEARER_TOKEN:-}" ]; then
  CURL_OPTS+=(-H "Authorization: Bearer ${BEARER_TOKEN}")
fi

query_result_count() {
  local query="$1"
  curl "${CURL_OPTS[@]}" -G --data-urlencode "query=${query}" \
    "${THANOS_URL}/api/v1/query" \
    | jq '.data.result | length' 2>/dev/null || echo 0
}

wait_for_metric() {
  local query="$1" label="$2"
  echo "Waiting for $label ($query) ..."
  for i in $(seq 1 "$MAX_ATTEMPTS"); do
    count=$(query_result_count "$query")
    if [ "$count" -gt 0 ]; then
      echo "  $label: $count series available"
      return 0
    fi
    echo "  attempt $i/$MAX_ATTEMPTS: waiting for $label ..."
    sleep "$POLL_INTERVAL"
  done
  echo "FAIL: $label returned no data after $((MAX_ATTEMPTS * POLL_INTERVAL))s"
  return 1
}

# Phase 1: wait for any scrape data via the generic "up" metric.
wait_for_metric "up" "scrape data"

# Phase 2: wait for each additional metric the caller requires.
for metric in "${EXTRA_METRICS[@]+"${EXTRA_METRICS[@]}"}"; do
  wait_for_metric "$metric" "$metric"
done

# Phase 3: accumulate enough history for range queries (since: 3m + buffer).
echo "Waiting ${HISTORY_WAIT}s for range query coverage (since: 3m + buffer)..."
sleep "$HISTORY_WAIT"
echo "Scrape history ready"
