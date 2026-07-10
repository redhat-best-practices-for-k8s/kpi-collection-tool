# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.

## Repository Overview

The KPI Collection Tool is a CLI application for automating metrics gathering and visualization for KPIs in disconnected environments. It collects metrics from Prometheus/Thanos endpoints on Kubernetes/OpenShift clusters and stores them in a database (SQLite or PostgreSQL) for analysis and visualization via Grafana.

### Key Features

- **Kubernetes Auto-Discovery**: Automatically discovers Thanos URL and creates service account tokens from kubeconfig
- **Manual Authentication**: Supports direct bearer token and Thanos URL configuration
- **Multiple Database Backends**: SQLite (default, local storage) and PostgreSQL (production deployments)
- **Flexible Sampling**: Configurable frequency and duration per KPI query
- **Dynamic CPU Placeholders**: Supports `{{RESERVED_CPUS}}` and `{{ISOLATED_CPUS}}` placeholders fetched from PerformanceProfile CRs
- **Grafana Integration**: Built-in Grafana dashboard management via Docker
- **Multi-format Output**: Table, JSON, and CSV output formats

## Build Commands

```bash
make build                      # Build statically linked binary (default, portable)
make build-darwin-arm64         # Build static binary for macOS ARM64 (Apple Silicon)
make build-debug                # Build static binary with debug symbols (for dlv/gdb)
make build-dynamic-linking      # Build dynamically linked binary
make build-dynamic-linking-debug # Build dynamic binary with debug symbols
make install                    # Install to ~/go/bin/kpi-collector
make uninstall                  # Remove from ~/go/bin
make install-kpi-collector      # Install from upstream (no local source needed)
```

The default `build` target produces a statically linked binary with `CGO_ENABLED=0`. This ensures
portability across Linux distributions without glibc version dependencies.

## Testing

```bash
make test               # Run all tests with verbose output
go test ./... -v        # Alternative direct command
```

## Linting

```bash
make lint                       # Run golangci-lint (auto-installs if needed)
make install-golangci-lint      # Install golangci-lint (auto-detects OS)
make install-golangci-lint-mac  # Install golangci-lint via Homebrew (macOS)
make install-golangci-lint-linux # Install golangci-lint via go install (Linux/CI)
```

The project uses golangci-lint v2 with configuration in `golangci.yml`. Enabled linters include:
- errcheck, govet, staticcheck, ineffassign, unused (code correctness)
- gosec (security)
- funlen, gocyclo, goconst (code quality)
- misspell, lll (style)
- gofmt, goimports (formatting)

## Code Organization

```
cmd/kpi-collector/main.go   # Entry point → commands.Execute()
internal/
  collector/                # KPI collection orchestration (goroutines per frequency group)
  commands/                 # CLI commands (Cobra): run, db show/remove, grafana start/stop, kpis generate
  config/                   # InputFlags, Query, KPIs structs; YAML loading; CPU placeholder substitution
  database/                 # Database interface + SQLite/Postgres implementations
  kubernetes/               # Kubeconfig auth, Thanos discovery, PerformanceProfile CPU fetching
  logger/                   # File-based logging
  output/                   # Table/JSON/CSV formatters
  prometheus/               # Prometheus/Thanos query client
grafana-templates/          # Embedded dashboard JSON (sqlite + postgres variants)
kpi-profiles/               # Embedded KPI YAML profiles (ran, core, hub, basic, quickstart)
```

## Key Dependencies

### Primary Dependencies
- **github.com/spf13/cobra**: CLI framework
- **github.com/prometheus/client_golang**: Prometheus client library
- **github.com/prometheus/common**: Prometheus data types
- **k8s.io/client-go**: Kubernetes client
- **k8s.io/api, k8s.io/apimachinery**: Kubernetes API types
- **modernc.org/sqlite**: SQLite driver (pure Go, no CGO required)
- **github.com/lib/pq**: PostgreSQL driver

### Testing Dependencies
- **github.com/onsi/ginkgo/v2**: BDD testing framework
- **github.com/onsi/gomega**: Assertion library
- **github.com/testcontainers/testcontainers-go**: Container-based testing

## Development Guidelines

### Go Version
This project uses Go 1.26. Ensure your environment matches.

### Testing Framework
Tests use Ginkgo/Gomega BDD framework. Test files follow the pattern `*_test.go` with corresponding `*_suite_test.go` files for test suite setup.

### CLI Structure (Cobra)
- `kpi-collector kpis generate --profile <profile> [--uncategorized]`: Generate a KPI file for a cluster profile
- `kpi-collector run [--once]`: Collect KPI metrics
- `kpi-collector db show clusters|kpis|categories|errors`: Query stored data
- `kpi-collector db remove clusters|kpis|errors`: Delete data
- `kpi-collector grafana start|stop`: Manage Grafana dashboard

### Database Interface
New backends implement the `Database` interface in `internal/database/interface.go`.

### Configuration
- Default artifacts directory: `./kpi-collector-artifacts/` (created in the current working directory)
- Override with `--artifacts-dir` flag (available on all commands); the provided path is used directly
- SQLite database: `<artifacts-dir>/kpi_metrics.db`
- Log files: `<artifacts-dir>/kpi-<timestamp>.log`
- Output files: `<artifacts-dir>/kpi-output-<timestamp>.json`
- Grafana config directory: `<artifacts-dir>/grafana/`
- Environment variables: `KPI_COLLECTOR_DB_TYPE`, `KPI_COLLECTOR_DB_URL`
- All commands (`run`, `db`, `grafana`) must be executed from the same working directory when using SQLite, or use `--artifacts-dir`

### KPI Configuration File Format
KPIs are defined in YAML format (see `kpis.yaml.template`):
```yaml
kpis:
  - id: unique-kpi-id
    promquery: your_promql_query
    # Optional: override global frequency (duration string or seconds)
    sample-frequency: 2m
    # Optional: route to a dedicated kpi_<category> table for better query performance
    category: cpu
    # Optional: collect this query only once
    run-once: true
    # Optional: instant (default) or range
    query-type: range
    # Required when query-type is range
    range:
      step: 30s    # Resolution between data points (required)
      since: 1h    # Start of the window (required)
      until: 30m   # End of the window (optional, defaults to "now")
```

Range query notes:
- `sample-frequency` controls how often the collector executes this KPI.
- `range.step` controls point spacing within each query result (required for range queries).
- `range.since` defines the start of the query window (required for range queries). Accepts
  either a Go duration string (e.g. `"2h"`, `"1m30s"`) interpreted as relative to "now", or
  an RFC 3339 timestamp (e.g. `"2026-04-07T12:24:25Z"`).
- `range.until` defines the end of the query window (optional, defaults to "now"). Accepts
  the same formats as `since`. Examples:
  - `"since": "2h"` → from 2 hours ago to now
  - `"since": "2h", "until": "1h"` → from 2 hours ago to 1 hour ago
  - `"since": "2026-04-07T12:00:00Z", "until": "2026-04-08T12:00:00Z"` → fixed window
  - `"since": "2h", "until": "2026-04-12T23:20:50Z"` → mixed relative/absolute
- PromQL windows such as `rate(...[5m])` still control the lookback window used per computed point.

### Error Handling
- Query errors are tracked in the database with error counts
- Use `db show errors` to view failed queries
- Use `db remove errors` to clear error counts

### Code Quality Requirements
- Functions should not exceed 60 lines (funlen)
- Cyclomatic complexity limit: 20
- Line length limit: 250 characters
- All exported functions should be documented

## Common Workflows

```bash
# Collect via kubeconfig (auto-discovers Thanos, creates token)
kpi-collector run --cluster-name my-cluster --cluster-type ran \
  --kubeconfig ~/.kube/config --kpis-file kpis.yaml --frequency 60 --duration 1h

# Collect via manual credentials
kpi-collector run --cluster-name my-cluster --cluster-type core \
  --token $TOKEN --thanos-url $THANOS_URL --kpis-file kpis.yaml

# Single collection pass
kpi-collector run --cluster-name my-cluster --cluster-type ran \
  --kubeconfig ~/.kube/config --kpis-file kpis.yaml --once

# Query stored data
kpi-collector db show clusters
kpi-collector db show kpis --name "node-cpu-usage" --cluster-name "my-cluster" --limit 100

# Grafana
kpi-collector grafana start --datasource=sqlite
kpi-collector grafana start --datasource=postgres --postgres-url "postgresql://user:pass@host:5432/kpi"
kpi-collector grafana stop
```

## Architecture Notes

### Collection Flow
Flags → load KPI YAML → discover Thanos (if kubeconfig) → substitute CPU placeholders →
group KPIs by frequency → spawn goroutines per group → query Prometheus → store in DB →
repeat until duration expires or `--once`.

### Database Schema
Tables: clusters (id, name, type, created), kpi_metrics (cluster_id, kpi_id, value, timestamp, exec_time, labels JSON), query_errors (kpi_id, error_count). Category queries go to `kpi_<category>` tables.

### Grafana Integration
Manages Grafana via Docker/Podman. Generates datasource config, provisions embedded dashboard templates, sets home dashboard via `GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH`.
