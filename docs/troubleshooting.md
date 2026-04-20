# Troubleshooting

Common issues and how to resolve them.

## "No data" or missing database — wrong working directory

**Symptom:** `db show` or `grafana start` returns no results even though collection completed successfully.

**Cause:** The tool stores all artifacts (SQLite database, logs, output) in `./kpi-collector-artifacts/` relative to your current working directory. If you run commands from a different directory, the tool looks for the database in the wrong place.

**Fix:** Either run all commands from the same directory, or use `--artifacts-dir` to point to the directory where collection artifacts were stored:

```bash
kpi-collector db show clusters --artifacts-dir /path/to/kpi-collector-artifacts
```

## TLS certificate errors

**Symptom:** `x509: certificate signed by unknown authority`

**Fix:** Add `--insecure-tls` to skip certificate verification:

```bash
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --kubeconfig ~/.kube/config \
  --kpis-file kpis.json \
  --insecure-tls
```

This is common with self-signed certificates, lab clusters, disconnected
environments, or kubeconfig files that don't include the CA bundle.

## Token expired during collection

**Symptom:** Queries start failing partway through a long collection run with authentication errors.

**Cause:** When using `--token` manually, the token may expire before the collection finishes.

**Fix:** When using `--kubeconfig`, the tool automatically sets the token expiry to match your `--duration` plus a 10-minute buffer, so this should not happen. If using manual `--token`, make sure the token duration covers your full collection run:

```bash
export TOKEN=$(oc create token prometheus-k8s -n openshift-monitoring --duration=2h)
```

## No PerformanceProfile found

**Symptom:** `no PerformanceProfile found in cluster`

**Cause:** Your KPI queries contain `{{RESERVED_CPUS}}` or `{{ISOLATED_CPUS}}` placeholders, but the cluster does not have a PerformanceProfile CR installed.

**Fix:** Either:
- Remove the placeholder queries from your `kpis.json` file
- Install the Node Tuning Operator and create a PerformanceProfile on the cluster
- Hardcode the CPU IDs directly in your queries (see [Collecting Metrics — Manual CPU IDs](collecting-metrics.md#manual-alternative-obtaining-cpu-ids-without---kubeconfig))

## KPI validation errors

**Symptom:** `found N KPI validation error(s)` at startup.

**Common causes:**
- **Invalid JSON syntax** — missing commas, trailing commas, unescaped quotes.
  Validate with `python3 -m json.tool kpis.json` or `jq . kpis.json`.
- **Duplicate KPI IDs** — each `id` must be unique across all entries.
- **Missing required fields** — `id` and `promquery` are required for every KPI.
- **Range query missing step/range** — when `query-type` is `"range"`, both `step` and `range` must be provided.

## Empty results from `db show kpis`

**Symptom:** `kpi-collector db show kpis` returns no rows, even though collection completed successfully.

**Check:**
1. **Cluster name mismatch** — use `kpi-collector db show clusters` to see what cluster names are stored, then filter with `--cluster-name` using the exact name.
2. **Wrong working directory** — if you ran `kpi-collector run` from a different directory, the SQLite database is in that directory's `kpi-collector-artifacts/` folder. Either `cd` to that directory or use `--artifacts-dir` to point to the correct location.
3. **KPI name filter** — `--name` must match the `id` field from your KPI file exactly (case-sensitive).

## "No data" in Grafana dashboard

1. Ensure data was collected first with `kpi-collector run`
2. Check Grafana time range (top-right corner) — try "Last 24 hours" or "Last 7 days"
3. Verify the KPI dropdown has a selected value
4. For SQLite, ensure the database exists in the artifacts directory (default: `./kpi-collector-artifacts/kpi_metrics.db`), or use `--artifacts-dir`
5. For PostgreSQL, test datasource connectivity in Grafana (**Settings** -> **Data Sources**)

## PostgreSQL connection errors

1. Verify PostgreSQL is running: `psql -l`
2. Check the connection URL format:
   `postgresql://user:password@host:port/dbname?sslmode=disable`
3. For local PostgreSQL with Docker/Podman, use the appropriate hostname:
   - `host.docker.internal` on Mac/Windows
   - `172.17.0.1` on Linux
4. Test the connection directly: `psql "your-connection-url"`

## Thanos returns empty results

**Symptom:** Collection succeeds but all values are empty or zero.

**Common causes:**
- **Stale metrics** — Thanos has a default staleness window (usually 5 minutes). If the cluster's Prometheus hasn't scraped recently, queries may return empty. Wait a few minutes and try again.
- **Wrong metric name** — verify the metric exists on your cluster by running:

```bash
oc exec -n openshift-monitoring prometheus-k8s-0 -- \
  curl -s 'http://localhost:9090/api/v1/label/__name__/values' | \
  python3 -m json.tool | grep "your_metric_name"
```

- **Label mismatch** — labels like `job`, `namespace`, or `instance` vary between clusters. Check the actual label values before using them in queries.

