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
make lint               # Run golangci-lint (auto-installs if needed)
```

The project uses golangci-lint v2 with configuration in `golangci.yml`. Enabled linters include:
- errcheck, govet, staticcheck, ineffassign, unused (code correctness)
- gosec (security)
- funlen, gocyclo, goconst (code quality)
- misspell, lll (style)
- gofmt, goimports (formatting)

## Code Organization

```
kpi-collection-tool/
├── cmd/kpi-collector/        # Main application entry point
│   └── main.go              # Calls commands.Execute()
├── internal/                 # Private packages
│   ├── collector/           # KPI collection orchestration
│   ├── commands/            # CLI commands (Cobra)
│   │   ├── root.go          # Root command
│   │   ├── run.go           # 'run' command - collect metrics
│   │   ├── db.go            # 'db' command - database operations
│   │   ├── db_show.go       # 'db show' subcommands
│   │   ├── db_remove.go     # 'db remove' subcommands
│   │   ├── grafana.go       # 'grafana' command
│   │   ├── grafana_start.go # 'grafana start' subcommand
│   │   └── grafana_stop.go  # 'grafana stop' subcommand
│   ├── config/              # Configuration types and validation
│   │   ├── types.go         # InputFlags, Query, KPIs structs
│   │   ├── cli_flags.go     # Flag validation
│   │   ├── kpis_loader.go   # KPI YAML file loading
│   │   └── query_placeholders.go  # CPU placeholder substitution
│   ├── database/            # Database abstraction layer
│   │   ├── interface.go     # Database interface definition
│   │   ├── factory.go       # Database factory/initialization
│   │   ├── sqlite.go        # SQLite implementation
│   │   └── postgres.go      # PostgreSQL implementation
│   ├── kubernetes/          # Kubernetes/OpenShift integration
│   │   ├── client.go        # Kubeconfig auth, Thanos discovery
│   │   ├── performanceprofile.go  # CPU fetching from PerformanceProfiles
│   │   └── types.go         # K8s client interface
│   ├── logger/              # File-based logging
│   ├── output/              # Multi-format output (table/json/csv)
│   │   ├── output.go        # Printer and record types
│   │   ├── table.go         # Table formatting
│   │   ├── json.go          # JSON formatting
│   │   └── csv.go           # CSV formatting
│   └── prometheus/          # Prometheus/Thanos client
│       ├── client.go        # Query execution and storage
│       └── types.go         # Token round tripper
├── grafana-templates/        # Embedded Grafana dashboard templates
│   ├── embed.go             # Go embed directives
│   ├── sqlite-dashboard.json
│   └── postgres-dashboard.json
├── kpis.yaml.template       # Example KPI configuration file
├── golangci.yml             # Linter configuration
└── Makefile                 # Build automation
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
This project uses Go 1.26.1. Ensure your environment matches.

### Testing Framework
Tests use Ginkgo/Gomega BDD framework. Test files follow the pattern `*_test.go` with corresponding `*_suite_test.go` files for test suite setup.

### CLI Structure
Commands are organized using Cobra:
- `kpi-collector kpis generate --profile <profile>`: Generate a KPI file for a cluster profile (ran, core, hub). Use `--uncategorized` to omit category fields. Use `--uncategorized` to omit category fields
- `kpi-collector run`: Collect KPI metrics (use `--once` to collect once and exit)
- `kpi-collector db show clusters|kpis|errors`: Query stored data
- `kpi-collector db remove clusters|kpis|errors`: Delete data
- `kpi-collector grafana start|stop`: Manage Grafana dashboard

### Database Interface
New database backends should implement the `Database` interface in `internal/database/interface.go`:
```go
type Database interface {
    InitDB() (*sql.DB, error)
    GetOrCreateCluster(db *sql.DB, clusterName string, clusterType string) (int64, error)
    IncrementQueryError(db *sql.DB, kpiID string) error
    GetQueryErrorCount(db *sql.DB, kpiID string) (int, error)
    StoreQueryResults(db *sql.DB, clusterID int64, queryID string, result model.Value) error
}
```

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

### Collecting Metrics from a Cluster
```bash
# Using kubeconfig (auto-discovers Thanos and creates token)
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --kubeconfig ~/.kube/config \
  --kpis-file kpis.yaml \
  --frequency 60 \
  --duration 1h

# Using manual credentials
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type core \
  --token $TOKEN \
  --thanos-url $THANOS_URL \
  --kpis-file kpis.yaml

# Single run: collect all KPIs once and exit
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --kubeconfig ~/.kube/config \
  --kpis-file kpis.yaml \
  --once
```

### Querying Stored Data
```bash
# List clusters
kpi-collector db show clusters

# Query KPIs with filters
kpi-collector db show kpis --name "node-cpu-usage" --cluster-name "my-cluster" --limit 100

# View query errors
kpi-collector db show errors
```

### Starting Grafana Dashboard
```bash
# With SQLite (default)
kpi-collector grafana start --datasource=sqlite

# With PostgreSQL
kpi-collector grafana start --datasource=postgres \
  --postgres-url "postgresql://user:pass@host:5432/kpi"

# Stop Grafana
kpi-collector grafana stop
```

### Building
```bash
# Build portable static binary (default)
make build

# Build with debug symbols (for use with dlv/gdb)
make build-debug

# Install globally
make install
```

## Architecture Notes

### Collection Flow
1. CLI parses flags and loads KPI configuration from YAML file
2. If kubeconfig provided, discovers Thanos URL and creates service account token
3. CPU placeholders are substituted if detected in queries
4. KPIs are grouped by sampling frequency
5. Goroutines are spawned per frequency group
6. Each goroutine executes queries at its frequency using Prometheus client
7. Results are stored in the configured database
8. Collection continues until duration expires or interrupted

### Database Schema
The database stores:
- **Clusters**: ID, name, type, created timestamp
- **KPI Metrics**: Cluster ID, KPI ID, value, timestamp, execution time, labels (JSON)
- **Query Errors**: KPI ID, error count

### Grafana Integration
The tool manages Grafana via Docker/Podman:
- Generates datasource configuration for SQLite or PostgreSQL
- Provisions pre-built dashboards from embedded templates
- Sets the KPI dashboard as the home page on login (`GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH`)
- Handles container lifecycle (start/stop)

Dashboard templates include hidden variables for "Fit to data" (time range auto-adjustment)
and inline statistics (sample count, average, min, max) displayed as a text panel.
