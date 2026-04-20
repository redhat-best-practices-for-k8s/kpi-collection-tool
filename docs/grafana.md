# Grafana

View collected KPI metrics in Grafana with a pre-configured dashboard.

The `grafana` command manages a local Grafana instance via Docker (or Podman). Configuration files are generated in `<artifacts-dir>/grafana/` (default: `./kpi-collector-artifacts/`). When using SQLite, run `grafana start` from the same directory where `kpi-collector run` was executed, or use `--artifacts-dir` to point to the artifacts directory.

Related guides:
- [Collecting Metrics](collecting-metrics.md)
- [Database Commands](database-commands.md)
- [Troubleshooting](troubleshooting.md)

## Quick Start

### Start Grafana

```bash
# Using SQLite (default)
kpi-collector grafana start --datasource=sqlite

# Using PostgreSQL
kpi-collector grafana start --datasource=postgres \
  --postgres-url "postgresql://user:password@host:5432/dbname"

# Custom port
kpi-collector grafana start --datasource=sqlite --port 3001
```

### Stop Grafana

```bash
kpi-collector grafana stop
```

## Command Reference

### `grafana start`

Start a local Grafana instance with the KPI dashboard pre-configured.

```bash
kpi-collector grafana start --datasource=<sqlite|postgres> [flags]
```

Flags:

| Flag | Required | Description | Example |
|------|----------|-------------|---------|
| `--datasource` | Yes | Database type: `sqlite` or `postgres` | `--datasource=postgres` |
| `--postgres-url` | If postgres | PostgreSQL connection string | `--postgres-url="postgresql://user:pass@host:5432/db"` |
| `--port` | No | Grafana port (default: `3000`) | `--port=3001` |

### `grafana stop`

Stop and remove the running Grafana container.

```bash
kpi-collector grafana stop
```

## PostgreSQL Connection URLs

When using PostgreSQL as the datasource, provide a connection URL in one of these formats.

Standard format:

```bash
postgresql://username:password@host:port/database
```

With SSL:

```bash
postgresql://username:password@host:port/database?sslmode=require
```

Without password:

```bash
postgresql://username@host:port/database
```

### Important: Docker Networking

Since Grafana runs in Docker, use the correct hostname:

| PostgreSQL Location | Hostname to Use |
|---------------------|-----------------|
| Mac/Windows host | `host.docker.internal` |
| Linux host | `172.17.0.1` |
| Docker container | Container name or IP |
| Remote server | Server hostname/IP |

Examples:

```bash
# PostgreSQL on your Mac/Windows machine
kpi-collector grafana start --datasource=postgres \
  --postgres-url "postgresql://user@host.docker.internal:5432/kpi_metrics"

# PostgreSQL on Linux host
kpi-collector grafana start --datasource=postgres \
  --postgres-url "postgresql://user@172.17.0.1:5432/kpi_metrics"

# Remote PostgreSQL server
kpi-collector grafana start --datasource=postgres \
  --postgres-url "postgresql://user:pass@db.example.com:5432/kpi_metrics"
```

## Accessing Grafana

1. Open `http://localhost:3000` (or your custom port)
2. Login:
   - Username: `admin`
   - Password: `admin`
   - You will be prompted to change the password on first login
3. Open dashboard: **Dashboards** -> **KPI Collection Tool - Dynamic Dashboard**

## Dashboard Features

### Dashboard Filters

Supported for both SQLite and PostgreSQL datasources:

- Cluster Name
- Cluster Type
- KPI

Additional filters available only for SQLite:

- Node
- Pod
- Job
- Container

All filters default to `All`.

The dashboard includes:

- Dynamic metric selection
- Time-series visualization
- Statistical summary (average, min, max, sample count)
- Detailed metrics table
- Query error tracking
- Multi-cluster support
