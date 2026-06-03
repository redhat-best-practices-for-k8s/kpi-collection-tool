#!/usr/bin/env bash
# Verification script for kpi-collector e2e tests.
# Usage: hack/verify-e2e.sh <scenario> [db-flags...]
#
# Scenarios:
#   once     - verify --once collection with range queries (kpis-e2e-once.yaml)
#   periodic - verify periodic collection with frequency override (kpis-e2e-periodic.yaml)
#   postgres - same as 'once' but expects extra db-flags for PostgreSQL
#
# Examples:
#   hack/verify-e2e.sh once
#   hack/verify-e2e.sh periodic
#   hack/verify-e2e.sh postgres --db-type postgres --postgres-url "$POSTGRES_URL"

set -euo pipefail

SCENARIO="${1:?Usage: verify-e2e.sh <scenario> [db-flags...]}"
shift
DB_FLAGS=("$@")

PASS_COUNT=0
FAIL_COUNT=0
BIN="./kpi-collector"

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  echo "  PASS: $1"
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  echo "  FAIL: $1"
}

kpi_count() {
  local name="$1"
  ${BIN} db show kpis --name "$name" -o json "${DB_FLAGS[@]}" | jq 'length'
}

category_count() {
  local cat="$1"
  ${BIN} db show kpis --category "$cat" -o json "${DB_FLAGS[@]}" | jq 'length'
}

check_min() {
  local name="$1" min="$2"
  local actual
  actual=$(kpi_count "$name")
  if [ "$actual" -ge "$min" ]; then
    pass "$name: $actual rows (>= $min)"
  else
    fail "$name: expected >= $min rows, got $actual"
  fi
}

check_exact() {
  local name="$1" expected="$2"
  local actual
  actual=$(kpi_count "$name")
  if [ "$actual" -eq "$expected" ]; then
    pass "$name: $actual rows (== $expected)"
  else
    fail "$name: expected exactly $expected rows, got $actual"
  fi
}

check_total() {
  local expected="$1"
  local actual
  actual=$(${BIN} db show kpis -o json "${DB_FLAGS[@]}" | jq 'length')
  if [ "$actual" -eq "$expected" ]; then
    pass "total rows: $actual (== $expected)"
  else
    fail "total rows: expected $expected, got $actual"
  fi
}

check_cluster() {
  local name="$1"
  local output
  output=$(${BIN} db show clusters "${DB_FLAGS[@]}")
  if echo "$output" | grep -q "$name"; then
    pass "cluster '$name' found"
  else
    fail "cluster '$name' not found in: $output"
  fi
}

check_category_exists() {
  local cat="$1"
  local output
  output=$(${BIN} db show categories "${DB_FLAGS[@]}")
  if echo "$output" | grep -q "$cat"; then
    pass "category '$cat' found"
  else
    fail "category '$cat' not found in: $output"
  fi
}

check_category_min() {
  local cat="$1" min="$2"
  local actual
  actual=$(category_count "$cat")
  if [ "$actual" -ge "$min" ]; then
    pass "category $cat: $actual rows (>= $min)"
  else
    fail "category $cat: expected >= $min rows, got $actual"
  fi
}

check_category_exact() {
  local cat="$1" expected="$2"
  local actual
  actual=$(category_count "$cat")
  if [ "$actual" -eq "$expected" ]; then
    pass "category $cat: $actual rows (== $expected)"
  else
    fail "category $cat: expected exactly $expected rows, got $actual"
  fi
}

check_no_errors() {
  local output
  output=$(${BIN} db show errors "${DB_FLAGS[@]}")
  if echo "$output" | grep -q "No errors found."; then
    pass "no query errors"
  else
    fail "unexpected errors: $output"
  fi
}

check_greater() {
  local name_a="$1" name_b="$2"
  local count_a count_b
  count_a=$(kpi_count "$name_a")
  count_b=$(kpi_count "$name_b")
  if [ "$count_a" -gt "$count_b" ]; then
    pass "$name_a ($count_a) > $name_b ($count_b)"
  else
    fail "$name_a ($count_a) should be > $name_b ($count_b)"
  fi
}

# Expected collection counts for periodic mode (--frequency 15s --duration 45s).
# From collector code: calculateTotalSamples = (durationSecs / frequencySecs) + 1
PERIODIC_GLOBAL_COUNT=4   # (45/15) + 1
PERIODIC_OVERRIDE_COUNT=5 # (45/10) + 1

echo "=== verify-e2e.sh: scenario=$SCENARIO ==="

case "$SCENARIO" in
  once)
    check_cluster "e2e-test"
    check_no_errors
    check_min "cpu-usage-range" 5
    check_min "memory-usage-range" 3
    check_min "pod-count-range" 5
    check_category_exists "workload"
    check_category_min "workload" 5
    ;;

  periodic)
    check_cluster "e2e-test"
    check_no_errors
    check_exact "node-cpu-usage" "$PERIODIC_GLOBAL_COUNT"
    check_exact "node-load-avg" "$PERIODIC_OVERRIDE_COUNT"
    check_exact "memory-available-avg" "$PERIODIC_GLOBAL_COUNT"
    check_greater "node-load-avg" "node-cpu-usage"
    check_category_exists "memory"
    check_category_exact "memory" "$PERIODIC_GLOBAL_COUNT"
    check_total $(( 2 * PERIODIC_GLOBAL_COUNT + PERIODIC_OVERRIDE_COUNT ))
    ;;

  postgres)
    check_cluster "e2e-test"
    check_no_errors
    check_min "cpu-usage-range" 5
    check_min "memory-usage-range" 3
    check_min "pod-count-range" 5
    check_category_exists "workload"
    check_category_min "workload" 5
    ;;

  *)
    echo "Unknown scenario: $SCENARIO"
    echo "Valid scenarios: once, periodic, postgres"
    exit 1
    ;;
esac

echo ""
echo "=== Results: $PASS_COUNT passed, $FAIL_COUNT failed ==="

if [ "$FAIL_COUNT" -gt 0 ]; then
  exit 1
fi
