# Getting Started

This guide walks you through installing kpi-collector, collecting your first metrics, and verifying the results. By the end you will have real data from your cluster stored locally.

## Prerequisites

Before you begin, make sure you have:

- **Access to an OpenShift cluster** via one of:
  - A kubeconfig file (`~/.kube/config` or a custom path)
  - A bearer token and Thanos querier URL (see [Collecting Metrics](collecting-metrics.md) for how to obtain these)
- **(Optional)** Docker or Podman, if you want to visualize data in Grafana later

## Step 1: Install

Follow the [Installation](installation.md) guide to download and set up the binary.

Verify:

```bash
kpi-collector --help
```

> [!NOTE]
> All examples in this documentation assume `kpi-collector` is on your
> PATH. If you prefer not to move it, replace `kpi-collector` with the
> full path to the binary wherever it appears.

## Step 2: Prepare a KPI file

kpi-collector reads a JSON file that tells it **which Prometheus metrics to collect**. Each entry has a unique `id` (the name you'll see in the database) and a `promquery` (a [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) expression that Thanos will evaluate).

The fastest way to create one is with the built-in generator. Pick the profile that matches your cluster type (`ran`, `core`, or `hub`):

```bash
# Generate all KPIs for a RAN cluster
kpi-collector kpis generate --profile ran --all

# Or interactively select which KPI categories to include
kpi-collector kpis generate --profile ran
```

This creates a `ran-kpis.json` file in your current directory with battle-tested PromQL queries for that profile. Use `-f <path>` to write to a custom location.

**Alternatively**, you can write a KPI file by hand. Save the following as `my-kpis.json` — a minimal example with two basic cluster-health queries:

```json
{
    "kpis": [
        {
            "id": "targets-healthy",
            "promquery": "up"
        },
        {
            "id": "pods-running",
            "promquery": "kubelet_running_pods"
        }
    ]
}
```

What these queries do:


| KPI               | What it measures                                                           |
| ----------------- | -------------------------------------------------------------------------- |
| `targets-healthy` | Whether each monitored target is up (1) or down (0) — one value per target |
| `pods-running`    | Number of running pods per node — changes as workloads scale up and down   |


See [KPI Configuration](kpis-file-configuration.md) for the full file format, all available fields, and the list of built-in profiles.

## Step 3: Collect metrics

Run a single collection to verify everything works:

```bash
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --kubeconfig /path/to/your/kubeconfig \
  --kpis-file my-kpis.json \
  --once \
  --insecure-tls
```

Replace `/path/to/your/kubeconfig` with your kubeconfig path, and `my-kpis.json` with `ran-kpis.json` if you used the generator in Step 2.

> [!NOTE]
> The `--insecure-tls` flag skips TLS certificate verification, which is
> common in lab and development clusters with self-signed certificates.
> Remove it if your cluster uses trusted certificates.

> [!TIP]
> If you don't have a kubeconfig, you can authenticate with a bearer token
> and Thanos URL instead — see [Collecting Metrics](collecting-metrics.md)
> for details.


| Flag             | Purpose                                                                                                                                          |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `--cluster-name` | A label you choose to identify this cluster in the database                                                                                      |
| `--cluster-type` | Category for grouping clusters — `ran`, `core`, or `hub`                                                                                         |
| `--kubeconfig`   | Path to a kubeconfig with access to the cluster. The tool uses it to auto-discover the Thanos URL and create a short-lived service account token |
| `--kpis-file`    | Path to the KPI JSON file you created in Step 2                                                                                                  |
| `--once`         | Collect every KPI once and exit (instead of repeating on a schedule)                                                                             |



Expected output:

```
KPI Collector starting...
Cluster name: my-cluster (type=ran)
Log file: kpi-collector-artifacts/kpi-2026-04-12-143000.log
Database: sqlite (kpi-collector-artifacts/kpi_metrics.db)
✓ Validated 2 KPI(s)
Discovered Thanos URL: thanos-querier-openshift-monitoring.apps.mycluster.example.com
Created service account token (sa=prometheus-k8s, ns=openshift-monitoring, duration=10m0s)

KPI Collection Started - Single run mode

[targets-healthy] Sample 1/1 (single run)
  Query: up
  Query Type: instant
  Status: OK - stored in database

[pods-running] Sample 1/1 (single run)
  Query: kubelet_running_pods
  Query Type: instant
  Status: OK - stored in database

KPI Collection Stopped: Single run completed
All queries completed successfully!
Artifacts stored in: /home/user/kpi-collector-artifacts
```

The tool creates a `kpi-collector-artifacts/` directory in your current working directory containing the SQLite database, logs, and output files. Use `--artifacts-dir` to store artifacts in a different location.

## Step 4: Verify the collected data

Check that your cluster was registered:

```bash
kpi-collector db show clusters
```

Expected output:

```
ID  CLUSTER_NAME   CREATED_AT            TOTAL_METRICS
--  ---            ---                   ---
1   my-cluster     2026-04-12 14:30:00   3
```

Query the collected KPI values:

```bash
kpi-collector db show kpis --name pods-running
```

Expected output:

```
ID  KPI_NAME      CLUSTER      VALUE   TIMESTAMP    EXECUTION_TIME        LABELS
--  ---           ---          ---     ---          ---                   ---
1   pods-running  my-cluster   47      1700000000   YYYY-MM-DD HH:MM:SS  {"instance":"node1"}
2   pods-running  my-cluster   32      1700000000   YYYY-MM-DD HH:MM:SS  {"instance":"node2"}

Total results: 2
```

If you see data, everything is working correctly.

## Next steps

- **Generate a full KPI profile** — use `kpi-collector kpis generate --profile ran --all` to create a comprehensive KPI file tailored to your cluster type. See [KPI Configuration](kpis-file-configuration.md).
- **Longer collection runs** — remove `--once` and use `--frequency 1m --duration 1h` to collect metrics over time. See [Collecting Metrics](collecting-metrics.md).
- **Write your own KPIs** — learn the KPI file format, per-query frequency overrides, and range queries in [KPI Configuration](kpis-file-configuration.md).
- **Visualize in Grafana** — launch a local Grafana dashboard with `kpi-collector grafana start --datasource=sqlite`. See [Grafana](grafana.md).
- **Query and manage data** — filter, sort, and export stored metrics with `kpi-collector db show`. See [Database Commands](database-commands.md).

