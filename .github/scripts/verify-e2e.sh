#!/usr/bin/env bash
# Verification script for kpi-collector e2e tests.
# Usage: .github/scripts/verify-e2e.sh <scenario> [db-flags...]
#
# Scenarios:
#   once     - verify --once collection with range queries (kpis-e2e-once.yaml)
#   periodic - verify periodic collection with frequency override (kpis-e2e-periodic.yaml)
#
# Examples:
#   .github/scripts/verify-e2e.sh once
#   .github/scripts/verify-e2e.sh periodic

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

# Expected row counts.
#
# Once mode (--once with range queries): since=3m, step=1m → 3/1 + 1 = 4 points per KPI.
ONCE_COUNT=4
#
# Periodic mode (--frequency 15s --duration 45s):
# From collector code: calculateTotalSamples = (durationSecs / frequencySecs) + 1
PERIODIC_GLOBAL_COUNT=4   # (45/15) + 1
PERIODIC_OVERRIDE_COUNT=5 # (45/10) + 1

echo "=== verify-e2e.sh: scenario=$SCENARIO ==="

case "$SCENARIO" in
  once)
    check_cluster "e2e-test"
    check_no_errors
    check_exact "node-load-range" "$ONCE_COUNT"
    check_exact "memory-usage-range" "$ONCE_COUNT"
    check_exact "pod-count-range" "$ONCE_COUNT"
    check_category_exists "workload"
    check_category_exact "workload" "$ONCE_COUNT"
    check_total $(( 3 * ONCE_COUNT ))
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

  *)
    echo "Unknown scenario: $SCENARIO"
    echo "Valid scenarios: once, periodic"
    exit 1
    ;;
esac

echo ""
echo "=== Results: $PASS_COUNT passed, $FAIL_COUNT failed ==="

if [ "$FAIL_COUNT" -gt 0 ]; then
  exit 1
fi
