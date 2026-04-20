# Database Commands

The `db` command provides direct access to query and manage collected KPI data stored in the database. It works with both SQLite (default) and PostgreSQL.


Related guides:
- [Collecting Metrics](collecting-metrics.md)
- [Grafana](grafana.md)
- [Troubleshooting](troubleshooting.md)

## Database Connection

You can specify the database connection in this order:

1. CLI flags: `--db-type`, `--postgres-url`
2. Environment variables: `KPI_COLLECTOR_DB_TYPE`, `KPI_COLLECTOR_DB_URL`
3. SQLite (used when no `--db-type` is specified): `<artifacts-dir>/kpi_metrics.db` (default: `./kpi-collector-artifacts/`)

Using environment variables:

```bash
# SQLite (default)
export KPI_COLLECTOR_DB_TYPE=sqlite

# PostgreSQL
export KPI_COLLECTOR_DB_TYPE=postgres
export KPI_COLLECTOR_DB_URL="postgresql://user:pass@localhost:5432/kpi?sslmode=disable"
```

Using CLI flags:

```bash
# SQLite (no flags needed)
kpi-collector db show clusters

# PostgreSQL
kpi-collector db show clusters \
  --db-type=postgres \
  --postgres-url="postgresql://user:pass@localhost:5432/kpi?sslmode=disable"
```

## Subcommands

The `db` command has two main subcommands:
- `show` for querying data
- `remove` for deleting data

## `db show`: Query Data

### Show Clusters

List all monitored clusters with creation dates and metric counts.

```bash
# List all clusters
kpi-collector db show clusters

# Filter by specific cluster
kpi-collector db show clusters --name="<cluster-name>"
```

Output:

```text
ID  CLUSTER_NAME     CREATED_AT           TOTAL_METRICS
--- ---              ---                  ---
1   <cluster-name>   2024-11-26 10:30:00  1,234
2   <cluster-name>   2024-11-26 09:15:00  567
```

### Show KPIs

Query and display KPI metrics with filters.

Basic usage:

```bash
# Show all metrics for a KPI
kpi-collector db show kpis --name="<kpi-name>"

# Filter by cluster
kpi-collector db show kpis --name="<kpi-name>" --cluster-name="<cluster-name>"
```

Advanced filtering:

```bash
# Filter by labels (exact match)
kpi-collector db show kpis --name="<kpi-name>" \
  --labels-filter='<label-key>=<label-value>'

# Time-based filtering (last 2 hours until 1 hour ago)
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

# Output as JSON
kpi-collector db show kpis --name="<kpi-name>" -o json

# Export to CSV file
kpi-collector db show kpis --name="<kpi-name>" -o csv > metrics.csv
```

Available flags:
- `--name`: KPI name to filter by
- `--cluster-name`: Cluster name to filter by
- `--labels-filter`: `<key>=<value>,<key2>=<value2>`
- `--since`: duration format like `2h`, `30m`, `24h`
- `--until`: duration format like `1h`, `15m`, `12h`
- `--limit`: maximum rows (`0` means no limit)
- `--sort`: `asc` or `desc` by execution time (default: `asc`)
- `-o`, `--output`: output format — `table` (default), `json`, or `csv`
- `--no-truncate`: show full label values without truncation

Output:

```text
ID   KPI_NAME       CLUSTER          VALUE      TIMESTAMP    EXECUTION_TIME       LABELS
---  ---            ---              ---        ---          ---                  ---
1    <kpi-name>     <cluster-name>   0.123456   1700000000   2024-11-26 10:30:00  {"<label-key>":"<label-value>"}
2    <kpi-name>     <cluster-name>   0.234567   1700000060   2024-11-26 10:31:00  {"<label-key>":"<label-value>"}

Total results: 2
```

### Show Errors

Display KPI queries that encountered errors during collection.

```bash
# List all query errors
kpi-collector db show errors
```

Output:

```text
KPI_ID          ERROR_COUNT
---             ---
<kpi-name-1>    5
<kpi-name-2>    2
```

## `db remove`: Delete Data

Warning: remove operations are immediate and cannot be undone.

### Remove Clusters

Delete a cluster record and all associated KPI metrics.

```bash
kpi-collector db remove clusters --name="<cluster-name>"
```

Output:

```text
Deleted cluster '<cluster-name>' and 1,234 metric samples.
```

### Remove KPIs

Delete KPI metrics from the database, optionally filtered by cluster and KPI name.

```bash
# Remove all KPIs from a cluster
kpi-collector db remove kpis --cluster-name="<cluster-name>"

# Remove specific KPI from a cluster
kpi-collector db remove kpis --cluster-name="<cluster-name>" --name="<kpi-name>"
```

Output:

```text
Deleted 567 metric samples.
```

### Remove Errors

Reset error counts for KPI queries.

```bash
# Clear errors for a specific KPI
kpi-collector db remove errors --name="<kpi-name>"

# Clear all errors
kpi-collector db remove errors --all
```

Output:

```text
Cleared 3 error record(s).
```

## Complete Examples

Using SQLite (default):

```bash
# Query clusters
kpi-collector db show clusters

# Query specific KPI
kpi-collector db show kpis --name="<kpi-name>" --limit=10

# Remove old cluster
kpi-collector db remove clusters --name="<cluster-name>"
```

Using PostgreSQL with environment variables:

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

Using PostgreSQL with flags:

```bash
# Each command needs connection flags
kpi-collector db show clusters \
  --db-type=postgres \
  --postgres-url="postgresql://kpiuser:pass@localhost:5432/kpi?sslmode=disable"

kpi-collector db show kpis --name="<kpi-name>" \
  --db-type=postgres \
  --postgres-url="postgresql://kpiuser:pass@localhost:5432/kpi?sslmode=disable"
```

## Database Support

SQLite is the default when no `--db-type` is specified.

### SQLite (default)

- No configuration required
- Data stored at `<artifacts-dir>/kpi_metrics.db` (default: `./kpi-collector-artifacts/`)
- Automatically created on first run
- No external dependencies
- All commands (`run`, `db`, `grafana`) must be executed from the same working directory, or use `--artifacts-dir` to specify the artifacts directory

### PostgreSQL

- Requires `--db-type postgres` and `--postgres-url`
- Requires PostgreSQL server (9.5+)
- URL formats:
  - `postgresql://user:password@host:port/dbname?sslmode=disable`
  - `host=host port=port user=user password=pass dbname=dbname sslmode=disable`
