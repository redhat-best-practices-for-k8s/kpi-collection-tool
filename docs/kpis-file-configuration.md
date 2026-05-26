# KPI Configuration

This guide covers the KPI YAML file format, sampling controls, and run modes.

Related guides:
- [Getting Started](getting-started.md)
- [Collecting Metrics](collecting-metrics.md)

## KPI File Format

KPIs are defined in a YAML file passed to `--kpis-file`. Each entry describes a PromQL query to execute against Prometheus/Thanos.

Minimal example:

```yaml
kpis:
  - id: node-cpu-usage
    promquery: avg by (instance) (rate(node_cpu_seconds_total{mode!="idle"}[5m]))
```

### Field Reference

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `id` | Yes | - | Unique identifier used in the database and output |
| `promquery` | Yes | - | PromQL query to execute |
| `sample-frequency` | No | global `--frequency` | Per-query override (duration string like `2m` or seconds like `120`) |
| `category` | No | — | Storage category; routes metrics to a dedicated `kpi_<category>` table for better query performance |
| `run-once` | No | false | Collect this query only once, skip repeated sampling |
| `query-type` | No | `instant` | `instant` or `range` |
| `range` | No* | — | Object with `step`, `since`, and optionally `until` |
| `range.step` | No* | — | Resolution between data points (e.g. `30s`) |
| `range.since` | No* | — | Start of the query window: Go duration (e.g. `1h`) or RFC 3339 timestamp |
| `range.until` | No | now | End of the query window: Go duration or RFC 3339 timestamp |

\* Required when `query-type` is `range`

### Full example with all fields

```yaml
kpis:
  - id: node-cpu-usage
    promquery: avg by (instance) (rate(node_cpu_seconds_total{mode!="idle"}[5m]))
    category: cpu

  - id: node-memory-usage
    promquery: node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes
    category: memory
    sample-frequency: 2m

  - id: cluster-uptime
    promquery: max(time() - process_start_time_seconds{job="kubelet"})
    run-once: true

  - id: node-cpu-usage-range
    promquery: avg by (instance) (rate(node_cpu_seconds_total{mode!="idle"}[5m]))
    category: cpu
    query-type: range
    range:
      step: 30s
      since: 1h
```

## Categories

The optional `category` field routes a KPI's metrics into a dedicated database table named `kpi_<category>` instead of the shared `query_results` table. This improves query performance at scale — when the database holds hundreds of thousands of rows, filtering by category avoids scanning unrelated data.

```yaml
kpis:
  - id: node-cpu-usage
    promquery: avg by (instance) (rate(node_cpu_seconds_total{mode!="idle"}[5m]))
    category: cpu

  - id: node-memory-usage
    promquery: node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes
    category: memory

  - id: cluster-uptime
    promquery: max(time() - process_start_time_seconds{job="kubelet"})
```

In this example, `node-cpu-usage` is stored in a `kpi_cpu` table, `node-memory-usage` in `kpi_memory`, and `cluster-uptime` in the default `query_results` table (no category).

How it works:

- **On write**: When the collector runs, it automatically creates the `kpi_<category>` table if it doesn't exist and registers the KPI in a `kpi_registry` lookup table.
- **On read**: `db show kpis` queries all tables (default + every category) and merges the results. Use `--category=cpu` to query only one category, or `--name=node-cpu-usage` to auto-discover the right table via the registry.
- **On delete**: `db remove kpis --category=cpu` removes all data from the category table and its registry entries.
- **Listing**: `db show categories` displays all registered categories, their backing tables, and KPI counts.

Category names are freeform strings (e.g. `cpu`, `memory`, `network`, `ptp`). The built-in KPI profiles (`kpis generate --profile`) include categories by default. Use `--uncategorized` to generate without them.

> [!NOTE]
> Once a KPI is stored with a category, you cannot change its category in a later run.
> The collector validates consistency at startup and will error if a category mismatch is detected.

## KPI Profiles

The tool includes built-in KPI profiles for common cluster types. Use the `kpis generate` command to create a ready-to-use file:

```bash
# Generate all KPIs for a RAN cluster
kpi-collector kpis generate --profile ran --all

# Interactively choose which categories to include
kpi-collector kpis generate --profile core

# Custom output path
kpi-collector kpis generate --profile hub --all -f /path/to/hub-kpis.yaml

# Overwrite an existing file
kpi-collector kpis generate --profile ran --all --overwrite

# Generate without category fields (all data stored in a single table)
kpi-collector kpis generate --profile ran --all --uncategorized
```

By default, generated KPIs include a `category` field that routes metrics into
per-category database tables for better query performance at scale. Use
`--uncategorized` to omit the `category` field from the output.

Available profiles:

| Profile | Use Case | KPIs |
|---------|----------|------|
| `ran` | RAN DU single-node clusters (reserved/isolated CPUs, hugepages, PTP, OVN) | 31 |
| `core` | Core clusters (control plane, etcd, API server, ingress, storage) | 22 |
| `hub` | Hub/ACM clusters (managed clusters, policy compliance, GitOps, etcd) | 22 |

The generated file can be edited freely — add, remove, or tweak queries to match your needs.

## Frequency and Duration

The `--frequency` and `--duration` flags control how metrics are collected over time. The number of samples collected equals `duration / frequency`.

- `--frequency`: how often to collect metrics (default: `1m`)
- `--duration`: total collection time before stopping (default: `45m`)

| Frequency | Duration | Samples | Use Case |
|-----------|----------|---------|----------|
| 60s | 45m | 45 | Default, balanced monitoring |
| 30s | 1h | 120 | Detailed analysis |
| 120s | 2h | 60 | Longer observation, less granularity |
| 10s | 5m | 30 | Quick troubleshooting |
| 300s (5m) | 24h | 288 | Long-term monitoring |


## Single Run Mode (`--once`)

Use `--once` to collect every KPI exactly once and exit. When set, `--frequency` and `--duration` are ignored.

Useful for:
- One-off snapshots of cluster metrics
- CI/CD pipelines that need a single data point
- Quick validation that queries and connectivity work
- Range queries that already aggregate over a time window

```bash
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --kubeconfig ~/.kube/config \
  --kpis-file kpis.yaml \
  --once
```

## Per-Query `run-once`

Individual queries can be marked with `run-once: true` in the KPI file. These execute once at the start of collection and are excluded from the repeated sampling loop, even when running with `--frequency` and `--duration`.

Useful for:
- Range queries that aggregate over a window (e.g. `rate(...[30m])`)
- Static cluster information (uptime, version, node count)
- Baseline snapshots before continuous monitoring begins

```yaml
kpis:
  - id: cluster-uptime
    promquery: max(time() - process_start_time_seconds{job="kubelet"})
    run-once: true

  - id: node-cpu-usage
    promquery: avg by (instance) (rate(node_cpu_seconds_total{mode!="idle"}[5m]))
```

In this example, `cluster-uptime` runs once immediately, while `node-cpu-usage` is sampled repeatedly at the configured frequency.

If all queries are marked `run-once: true`, the collector executes them all once and exits without waiting for the duration timer.

## Range Queries

Set `query-type: range` to execute a Prometheus range query instead of an instant query. Range queries return a series of data points over a time window.

```yaml
kpis:
  - id: node-cpu-range
    promquery: avg by (instance) (rate(node_cpu_seconds_total{mode!="idle"}[5m]))
    query-type: range
    range:
      step: 30s
      since: 1h
```

How the time controls relate:

- `sample-frequency` — how often the collector executes this query
- `range.since` / `range.until` — the query window (how far back, and optionally when to stop)
- `range.step` — spacing between data points in the returned range
- PromQL windows like `rate(...[5m])` control the per-point lookback, independent of the above

## Dynamic CPU IDs from PerformanceProfile CRs

Queries can use `{{RESERVED_CPUS}}` and `{{ISOLATED_CPUS}}` placeholders. These are replaced at startup with CPU IDs from PerformanceProfile CRs in the cluster. This feature requires `--kubeconfig` authentication.

```yaml
kpis:
  - id: cpu-reserved-set
    promquery: rate(node_cpu_seconds_total{cpu=~"{{RESERVED_CPUS}}"}[30m])
```

For full details on how CPU substitution works and how to obtain CPU IDs manually, see [Collecting Metrics — Dynamic CPU IDs](collecting-metrics.md#dynamic-cpu-ids-from-performanceprofile-crs).
