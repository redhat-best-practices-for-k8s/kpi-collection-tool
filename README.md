# KPI Collection Tool

Tool to automate metrics gathering and visualization for KPIs in disconnected environments.

## Installation

### Using Make (recommended)

```bash
# Build and install globally
make install

# This installs kpi-collector to ~/go/bin/
# Make sure ~/go/bin is in your PATH
```

To add `~/go/bin` to your PATH, add this to your `~/.zshrc` (macOS) or `~/.bashrc` (Linux):
```bash
export PATH="$HOME/go/bin:$PATH"
```

Then reload your shell:
```bash
source ~/.zshrc  # or source ~/.bashrc on Linux
```

### Uninstall

```bash
make uninstall
```

## Usage

The tool uses subcommands for different operations. Get help anytime with:

```bash
kpi-collector --help
kpi-collector run --help
kpi-collector version
```

## Collecting KPI Metrics

The `run` command gathers KPI metrics from Prometheus/Thanos and stores them in a database.

### Authentication Modes

#### 1. Using Kubeconfig (Automatic Discovery)

Automatically discovers Thanos URL and creates a service account token.

**Basic usage (uses SQLite by default):**
```bash
kpi-collector run \
  --cluster-name my-cluster \
  --kubeconfig ~/.kube/config
```

**With custom sampling parameters:**
```bash
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --kubeconfig ~/.kube/config \
  --frequency 30 \
  --duration 1h \
  --output my-metrics.json \
  --log my-app.log
```

**Explicitly using SQLite:**
```bash
kpi-collector run \
  --cluster-name my-cluster \
  --kubeconfig ~/.kube/config \
  --db-type sqlite
```

**Using PostgreSQL:**
```bash
kpi-collector run \
  --cluster-name my-cluster \
  --kubeconfig ~/.kube/config \
  --db-type postgres \
  --postgres-url "postgresql://myuser:mypass@localhost:5432/kpi_metrics?sslmode=disable"
```

#### 2. Using Manual Credentials

Provide Thanos URL and bearer token directly.

**Basic usage (uses SQLite by default):**
```bash
kpi-collector run \
  --cluster-name my-cluster \
  --token YOUR_BEARER_TOKEN \
  --thanos-url thanos-querier.example.com
```

**With custom sampling parameters:**
```bash
kpi-collector run \
  --cluster-name my-cluster \
  --token YOUR_BEARER_TOKEN \
  --thanos-url thanos-querier.example.com \
  --frequency 120 \
  --duration 30m \
  --output results.json
```

**Using PostgreSQL:**
```bash
kpi-collector run \
  --cluster-name my-cluster \
  --token YOUR_BEARER_TOKEN \
  --thanos-url thanos-querier.example.com \
  --db-type postgres \
  --postgres-url "postgresql://myuser:mypass@localhost:5432/kpi_metrics?sslmode=disable"
```
## --insecure-tls

Use this flag when running the tool against clusters or Prometheus/Thanos servers with self-signed or untrusted certificates.

```bash
./kpi-collector \
  --cluster-name my-cluster \
  --kubeconfig ~/.kube/config \
  --insecure-tls
```  
### What it does
- Skips TLS certificate verification for all HTTPS requests:
- Kubernetes API calls
- Thanos/Prometheus queries
- Allows execution in environments where the certificate cannot be validated.

### When to use
- Self-signed certificates
- Disconnected / lab / air-gapped clusters
- kubeconfig without a valid CA
- TLS errors such as:
  x509: certificate signed by unknown authority

### Complete Examples

**Development setup with SQLite (default):**
```bash
kpi-collector run \
  --cluster-name dev-cluster \
  --kubeconfig ~/.kube/config \
  --frequency 60 \
  --duration 1h \
  --insecure-tls
```

**Production setup with PostgreSQL:**
```bash
kpi-collector run \
  --cluster-name prod-cluster \
  --token YOUR_BEARER_TOKEN \
  --thanos-url thanos-querier.prod.example.com \
  --db-type postgres \
  --postgres-url "postgresql://kpi_user:secure_password@postgres.example.com:5432/kpi_metrics?sslmode=require" \
  --frequency 30 \
  --duration 24h \
  --output prod-metrics.json \
  --log prod-kpi.log
```

**Using a custom KPIs configuration file:**
```bash
kpi-collector run \
  --cluster-name my-cluster \
  --kubeconfig ~/.kube/config \
  --kpis-file /path/to/custom-kpis.json
```

## Command Line Flags

### Collect Command Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--cluster-name` | Yes | - | Name of the cluster being monitored |
| `--cluster-type` | No | - | Cluster type for categorization: `ran`, `core`, or `hub` |
| `--kubeconfig` | No* | - | Path to kubeconfig file for auto-discovery |
| `--token` | No* | - | Bearer token for Thanos authentication |
| `--thanos-url` | No* | - | Thanos querier URL (without https://) |
| `--insecure-tls` | No | false | Skip TLS certificate verification (dev only) |
| `--frequency` | No | 60 | Sampling frequency in seconds (how often to collect metrics) |
| `--duration` | No | 45m | Total duration for sampling (e.g. 10s, 1m, 2h, 24h) |
| `--output` | No | kpi-output.json | Output file name for results |
| `--log` | No | kpi.log | Log file name |
| `--db-type` | No | sqlite | Database type: `sqlite` or `postgres` |
| `--postgres-url` | No** | - | PostgreSQL connection string |
| `--kpis-file` | No | configs/kpis.json | Path to KPIs configuration file |

\* Either provide `--kubeconfig` OR both `--token` and `--thanos-url`

\*\* Required when `--db-type=postgres`

### Understanding Frequency and Duration

The `--frequency` and `--duration` flags work together to control how metrics are collected:

- **`--frequency`**: How often (in seconds) to collect metrics
  - Example: `--frequency 60` means collect metrics every 60 seconds (once per minute)
  - Lower values = more frequent sampling = more data points
  - Higher values = less frequent sampling = fewer data points

- **`--duration`**: How long to keep collecting metrics before stopping
  - Accepts time units: `s` (seconds), `m` (minutes), `h` (hours)
  - Example: `--duration 1h` means run for 1 hour total
  - The tool will automatically stop after this time period

**How they work together:**

The number of samples collected = `duration / frequency`

**Examples:**

| Frequency | Duration | Total Samples | Use Case |
|-----------|----------|---------------|----------|
| 60s | 45m | 45 samples | Default - balanced monitoring |
| 30s | 1h | 120 samples | More frequent sampling for detailed analysis |
| 120s | 2h | 60 samples | Less frequent, longer observation period |
| 10s | 5m | 30 samples | Quick test or troubleshooting |
| 300s (5m) | 24h | 288 samples | Long-term monitoring with less granularity |

**Choosing the right values:**

- **Development/Testing**: Short duration (5-10m), frequent sampling (30-60s)
  ```bash
  --frequency 30 --duration 10m
  ```

- **Production Monitoring**: Longer duration (1-24h), moderate sampling (60-120s)
  ```bash
  --frequency 60 --duration 24h
  ```

- **Troubleshooting**: Very frequent sampling (10-30s), short duration (5-15m)
  ```bash
  --frequency 10 --duration 5m
  ```

## Querying and Managing Data with `db` Command

The `db` command provides direct access to query and manage collected KPI data stored in the database. It works with both SQLite (default) and PostgreSQL databases.

### Database Connection

You can specify the database connection in three ways (in order of priority):

1. **CLI flags**: `--db-type` and `--postgres-url`
2. **Environment variables**: `KPI_COLLECTOR_DB_TYPE` and `KPI_COLLECTOR_DB_URL`
3. **Default**: SQLite at `~/.kpi-collector/kpi_metrics.db`

**Using environment variables (recommended):**
```bash
# For SQLite (default)
export KPI_COLLECTOR_DB_TYPE=sqlite

# For PostgreSQL
export KPI_COLLECTOR_DB_TYPE=postgres
export KPI_COLLECTOR_DB_URL="postgresql://user:pass@localhost:5432/kpi?sslmode=disable"
```

**Using CLI flags:**
```bash
# For SQLite (default - no flags needed)
kpi-collector db show clusters

# For PostgreSQL
kpi-collector db show clusters \
  --db-type=postgres \
  --postgres-url="postgresql://user:pass@localhost:5432/kpi?sslmode=disable"
```

### Subcommands

The `db` command has two main subcommands: `show` (query data) and `remove` (delete data).

### `db show` - Query Data

#### Show Clusters

List all monitored clusters with their creation dates and metric counts.

```bash
# List all clusters
kpi-collector db show clusters

# Filter by specific cluster
kpi-collector db show clusters --name="<cluster-name>"
```

**Output:**
```
ID  CLUSTER_NAME     CREATED_AT           TOTAL_METRICS
--- ---              ---                  ---
1   <cluster-name>   2024-11-26 10:30:00  1,234
2   <cluster-name>   2024-11-26 09:15:00  567
```

#### Show KPIs

Query and display KPI metrics with filtering options.

**Basic usage:**
```bash
# Show all metrics for a KPI
kpi-collector db show kpis --name="<kpi-name>"

# Filter by cluster
kpi-collector db show kpis --name="<kpi-name>" --cluster-name="<cluster-name>"
```

**Advanced filtering:**
```bash
# Filter by labels (exact match)
kpi-collector db show kpis --name="<kpi-name>" \
  --labels-filter='<label-key>=<label-value>'

# Time-based filtering (show metrics from last 2 hours until 1 hour ago)
kpi-collector db show kpis --name="<kpi-name>" --since="2h" --until="1h"

# Limit results and sort by execution time
kpi-collector db show kpis --name="<kpi-name>" --limit=100 --sort="desc"

# Combine multiple filters
kpi-collector db show kpis \
  --name="<kpi-name>" \
  --cluster-name="<cluster-name>" \
  --since="24h" \
  --limit=50 \
  --sort="desc"
```

**Available flags:**
- `--name`: KPI name to filter by
- `--cluster-name`: Cluster name to filter by
- `--labels-filter`: Label filters in format `<key>=<value>,<key2>=<value2>`
- `--since`: Show metrics since (duration format: `2h`, `30m`, `24h`)
- `--until`: Show metrics until (duration format: `1h`, `15m`, `12h`)
- `--limit`: Limit number of results (0 = no limit)
- `--sort`: Sort order by execution time: `asc` or `desc` (default: `asc`)

**Output:**
```
ID   KPI_NAME       CLUSTER          VALUE      TIMESTAMP    EXECUTION_TIME       LABELS
---  ---            ---              ---        ---          ---                  ---
1    <kpi-name>     <cluster-name>   0.123456   1700000000   2024-11-26 10:30:00  {"<label-key>":"<label-value>"}
2    <kpi-name>     <cluster-name>   0.234567   1700000060   2024-11-26 10:31:00  {"<label-key>":"<label-value>"}

Total results: 2
```

#### Show Errors

Display KPI queries that have encountered errors during collection.

```bash
# List all query errors
kpi-collector db show errors
```

**Output:**
```
KPI_ID          ERROR_COUNT
---             ---
<kpi-name-1>    5
<kpi-name-2>    2
```

### `db remove` - Delete Data

**WARNING:** All remove operations are immediate and cannot be undone.

#### Remove Clusters

Delete a cluster record and all associated KPI metrics.

```bash
# Remove a cluster and all its data
kpi-collector db remove clusters --name="<cluster-name>"
```

**Output:**
```
✓ Deleted cluster '<cluster-name>' and 1,234 metric samples.
```

#### Remove KPIs

Delete KPI metrics from the database, optionally filtered by cluster and KPI name.

```bash
# Remove all KPIs from a cluster
kpi-collector db remove kpis --cluster-name="<cluster-name>"

# Remove specific KPI from a cluster
kpi-collector db remove kpis --cluster-name="<cluster-name>" --name="<kpi-name>"
```

**Output:**
```
✓ Deleted 567 metric samples.
```

#### Remove Errors

Reset error counts for KPI queries.

```bash
# Clear errors for a specific KPI
kpi-collector db remove errors --name="<kpi-name>"

# Clear all errors
kpi-collector db remove errors --all
```

**Output:**
```
✓ Cleared 3 error record(s).
```

### Complete Examples

**Using SQLite (default):**
```bash
# Query clusters
kpi-collector db show clusters

# Query specific KPI
kpi-collector db show kpis --name="<kpi-name>" --limit=10

# Remove old cluster
kpi-collector db remove clusters --name="<cluster-name>"
```

**Using PostgreSQL with environment variables:**
```bash
# Set connection once
export KPI_COLLECTOR_DB_TYPE=postgres
export KPI_COLLECTOR_DB_URL="postgresql://kpiuser:pass@localhost:5432/kpi?sslmode=disable"

# Query data
kpi-collector db show clusters
kpi-collector db show kpis --name="<kpi-name>" --cluster-name="<cluster-name>"
kpi-collector db show errors

# Manage data
kpi-collector db remove kpis --cluster-name="<cluster-name>" --name="<kpi-name>"
```

**Using PostgreSQL with flags:**
```bash
# Each command needs the connection flags
kpi-collector db show clusters \
  --db-type=postgres \
  --postgres-url="postgresql://kpiuser:pass@localhost:5432/kpi?sslmode=disable"

kpi-collector db show kpis --name="<kpi-name>" \
  --db-type=postgres \
  --postgres-url="postgresql://kpiuser:pass@localhost:5432/kpi?sslmode=disable"
```

## Database Support

The tool supports two database backends. **SQLite is used by default** when no `--db-type` flag is specified.

### SQLite (Default)
- **Default behavior** - no configuration needed
- Data stored in: `~/.kpi-collector/kpi_metrics.db`
- Automatically created on first run
- No external dependencies
- Works from any directory

### PostgreSQL
- Requires `--db-type postgres` and `--postgres-url` flags
- Requires PostgreSQL server (version 9.5+)
- Connection URL formats:
  - **Standard URL**: `postgresql://user:password@host:port/dbname?sslmode=disable`
  - **Key-value**: `host=host port=port user=user password=pass dbname=dbname sslmode=disable`

# Visualizing Data with Grafana

View your collected KPI metrics in Grafana with a pre-configured dashboard.

The `grafana` command manages a local Grafana instance via Docker. Configuration files are automatically generated in `~/.kpi-collector/grafana/`.

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

**Flags:**

| Flag | Required | Description | Example |
|------|----------|-------------|---------|
| `--datasource` | Yes | Database type: `sqlite` or `postgres` | `--datasource=postgres` |
| `--postgres-url` | If postgres | PostgreSQL connection string | `--postgres-url="postgresql://user:pass@host:5432/db"` |
| `--port` | No | Grafana port (default: 3000) | `--port=3001` |

### `grafana stop`

Stop and remove the running Grafana container.

```bash
kpi-collector grafana stop
```

## PostgreSQL Connection URLs

When using PostgreSQL as the datasource, provide a connection URL in one of these formats:

**Standard Format:**
```bash
postgresql://username:password@host:port/database
```

**With SSL:**
```bash
postgresql://username:password@host:port/database?sslmode=require
```

**Without Password:**
```bash
postgresql://username@host:port/database
```

### Important: Docker Networking

Since Grafana runs in Docker, use the appropriate hostname:

| PostgreSQL Location | Hostname to Use |
|---------------------|----------------|
| **Mac/Windows Host** | `host.docker.internal` |
| **Linux Host** | `172.17.0.1` |
| **Docker Container** | Container name or IP |
| **Remote Server** | Server hostname/IP |

**Examples:**

```bash
# PostgreSQL on your Mac/Windows machine
kpi-collector grafana --datasource=postgres \
  --postgres-url "postgresql://user@host.docker.internal:5432/kpi_metrics"

# PostgreSQL on Linux host
kpi-collector grafana --datasource=postgres \
  --postgres-url "postgresql://user@172.17.0.1:5432/kpi_metrics"

# Remote PostgreSQL server
kpi-collector grafana --datasource=postgres \
  --postgres-url "postgresql://user:pass@db.example.com:5432/kpi_metrics"
```

## Accessing Grafana

1. **Open Browser:** http://localhost:3000 (or your custom port)
2. **Login:** 
   - Username: `admin`
   - Password: `admin`
   - You'll be prompted to change the password on first login
3. **View Dashboard:** Navigate to **Dashboards** → **KPI Collection Tool - Dynamic Dashboard**

## Dashboard Features

The dashboard includes:

- **Dynamic KPI Selection:** Choose any KPI from the dropdown
- **Time-series Visualization:** View metric values over time
- **Statistical Summary:** Average, min, max, sample count
- **Detailed Metrics Table:** All labels and values
- **Query Error Tracking:** Monitor failed queries
- **Multi-cluster Support:** Filter by cluster and cluster type

## Troubleshooting

### "No data" in dashboard

1. Ensure you've collected data first using `kpi-collector run`
2. Check the time range in Grafana (top-right corner) - try "Last 24 hours" or "Last 7 days"
3. Verify the KPI dropdown has a selection
4. For SQLite: Check that `~/.kpi-collector/kpi_metrics.db` exists
5. For PostgreSQL: Test the datasource connection in Grafana Settings → Data Sources

### PostgreSQL connection errors

1. Verify PostgreSQL is running: `psql -l`
2. Check the connection URL is correct
3. For local PostgreSQL with Docker, use `host.docker.internal` (Mac/Windows) or `172.17.0.1` (Linux)
4. Test connection: `psql "your-connection-url"`

