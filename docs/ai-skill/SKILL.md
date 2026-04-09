---
name: kpi-collector
description: Use and configure the kpi-collector CLI for collecting Prometheus/Thanos metrics from OpenShift clusters. Generates kpis.json files with Telco-specific PromQL queries for RAN, Core, PTP, networking, and resource compliance. Triggered when the user mentions kpi-collector, KPI collection, kpis.json, Telco metrics, PromQL for OpenShift, or Grafana dashboards for cluster monitoring.
---

# kpi-collector CLI Skill

## Quick Command Reference

| Action | Command |
|--------|---------|
| Collect metrics (kubeconfig) | `kpi-collector run --cluster-name NAME --cluster-type ran --kubeconfig ~/.kube/config --kpis-file kpis.json` |
| Collect metrics (manual auth) | `kpi-collector run --cluster-name NAME --cluster-type core --token $TOKEN --thanos-url $URL --kpis-file kpis.json` |
| Collect once and exit | `kpi-collector run --cluster-name NAME --kpis-file kpis.json --once` |
| List clusters | `kpi-collector db show clusters` |
| Show KPI metrics | `kpi-collector db show kpis --name "cpu-system" --cluster-name "mycluster"` |
| Show errors | `kpi-collector db show errors` |
| Remove KPIs | `kpi-collector db remove kpis --name "cpu-system"` |
| Remove cluster | `kpi-collector db remove clusters --name "mycluster"` |
| Clear errors | `kpi-collector db remove errors` |
| Start Grafana (SQLite) | `kpi-collector grafana start --datasource=sqlite` |
| Start Grafana (Postgres) | `kpi-collector grafana start --datasource=postgres --postgres-url "postgresql://user:pass@host:5432/db"` |
| Stop Grafana | `kpi-collector grafana stop` |

## Run Command Flags

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--cluster-name` | Yes | — | Cluster identifier |
| `--kpis-file` | Yes | — | Path to kpis.json |
| `--cluster-type` | No | — | `ran`, `core`, or `hub` |
| `--kubeconfig` | No* | — | Auto-discovers Thanos URL and creates token |
| `--token` | No* | — | Bearer token (manual auth) |
| `--thanos-url` | No* | — | Thanos URL without `https://` (manual auth) |
| `--frequency` | No | `60s` | Sampling interval (e.g. `30s`, `2m`) |
| `--duration` | No | `45m` | Total collection time (e.g. `1h`, `24h`) |
| `--once` | No | `false` | Collect all KPIs once and exit (mutually exclusive with `--frequency`/`--duration`) |
| `--db-type` | No | `sqlite` | `sqlite` or `postgres` |
| `--postgres-url` | No | — | Required when `--db-type=postgres` |
| `--insecure-tls` | No | `false` | Skip TLS verification |
| `--log` | No | `kpi.log` | Log file path |

*Either `--kubeconfig` or both `--token` + `--thanos-url` are required.

## Show KPIs Flags

| Flag | Description |
|------|-------------|
| `--name` | Filter by KPI ID |
| `--cluster-name` | Filter by cluster |
| `--labels-filter` | Label match: `key=value,key2=value2` |
| `--since` | Duration ago: `2h`, `30m`, `24h` |
| `--until` | Duration ago: `1h`, `15m` |
| `--limit` | Max results (0 = no limit) |
| `--sort` | `asc` or `desc` by execution time |
| `--no-truncate` | Show full labels |
| `-o` | Output format: `table`, `json`, `csv` |

## kpis.json File Format

When generating a kpis.json file, use this structure:

```json
{
    "kpis": [
        {
            "id": "unique-kpi-id",
            "promquery": "your_promql_query_here"
        }
    ]
}
```

### Supported fields per KPI entry

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `id` | Yes | — | Unique identifier (used in DB and output) |
| `promquery` | Yes | — | PromQL query string |
| `sample-frequency` | No | global `--frequency` | Override per-KPI (seconds or duration string like `"2m"`) |
| `run-once` | No | `false` | Collect once at start, skip repeated sampling |
| `query-type` | No | `"instant"` | `"instant"` or `"range"` |
| `range` | range only | — | Object with `step`, `since`, and optionally `until` |
| `range.step` | range only | — | Resolution between points (e.g. `"30s"`) |
| `range.since` | range only | — | Start of the window: Go duration (e.g. `"2h"`) or RFC 3339 timestamp |
| `range.until` | range only | now | End of the window: Go duration (e.g. `"1h"`) or RFC 3339 timestamp |

### Range query rules

- `sample-frequency` = how often the collector executes the query
- `range.since` = start of the query window (required); Go duration or RFC 3339 timestamp
- `range.until` = end of the query window (optional, defaults to "now"); same formats as `since`
- `range.step` = spacing between data points in the result
- Both `since` and `until` accept either a Go duration (`"2h"`, `"1m30s"`) interpreted relative to "now", or an RFC 3339 timestamp (`"2026-04-07T12:00:00Z"`)
- Examples: `"since": "2h"` (2h ago to now), `"since": "2h", "until": "1h"` (2h ago to 1h ago), `"since": "2026-04-07T12:00:00Z", "until": "2026-04-08T12:00:00Z"` (fixed window)
- PromQL windows like `rate(...[5m])` control the per-point lookback independently
- If `frequency > since` (when since is a duration), you get data gaps (the tool blocks this with an error)
- If `frequency < since/2` (when since is a duration), you get heavy overlap (the tool warns)

### Dynamic CPU placeholders

Use `{{RESERVED_CPUS}}` and `{{ISOLATED_CPUS}}` in promqueries. They are auto-replaced
with CPU IDs from PerformanceProfile CRs (e.g. `"0-1,32-33"` becomes `"0|1|32|33"`).
Requires `--kubeconfig` authentication.

## Before Running kpi-collector

Always gather details from the user before running any `kpi-collector run` command.
Ask for:

1. **Cluster name** — identifier for this cluster in the database
2. **Cluster type** — `ran`, `core`, or `hub` (optional)
3. **Authentication method**:
   - **Kubeconfig** — ask if `~/.kube/config` is correct, or get a custom path.
     Remind the user their kubeconfig credentials must be valid (e.g. `oc login` first).
   - **Token + Thanos URL** — ask for both the bearer token and the Thanos querier URL
     (without `https://` prefix). Use this when kubeconfig is unavailable or expired.
4. **Run mode** — single snapshot (`--once`) or continuous collection?
   If continuous, ask for:
   - **Frequency** — how often to sample (default: `60s`). Examples: `30s`, `2m`, `5m`
   - **Duration** — how long to run (default: `45m`). Examples: `1h`, `8h`, `24h`
5. **Database backend** — SQLite (default, local) or PostgreSQL?
   If PostgreSQL, ask for the connection string.
6. **TLS** — if targeting a lab or disconnected cluster, ask whether to skip
   TLS verification (`--insecure-tls`). Default is to verify.

If using the AskQuestion tool, structure it like:

```
- "How do you want to authenticate?" → ["Kubeconfig (auto-discovery)", "Bearer token + Thanos URL"]
- If kubeconfig: "Use default ~/.kube/config?" → ["Yes", "I'll provide a custom path"]
- If token: ask for token value and Thanos URL
- "Run mode?" → ["Collect once (--once)", "Continuous collection"]
- If continuous: "How often and for how long?" — let user specify or offer defaults
- "Database?" → ["SQLite (default, local file)", "PostgreSQL"]
- "Skip TLS verification?" → ["No (production)", "Yes (lab/self-signed certs)"]
```

## Generating kpis.json for Telco Workloads

When the user asks to create KPIs for a Telco cluster, ask which cluster type they are
monitoring: **RAN**, **Core**, or **Hub**. Then assemble a `kpis.json` using queries
from the matching section in [telco-promql.md](telco-promql.md), which is organized
by cluster type with complete ready-to-use KPI sets.

## Common Gotchas

1. **Thanos staleness**: Thanos deduplicates and compacts data. Queries with very short
   rate windows (e.g. `rate(...[1m])`) may return empty on Thanos. Use `[5m]` or wider.

2. **CPU placeholders need kubeconfig**: Using `{{RESERVED_CPUS}}` or `{{ISOLATED_CPUS}}`
   without `--kubeconfig` fails immediately.

3. **Range query frequency vs since**: If `--frequency` (or `sample-frequency`) is larger
   than `range.since` (when since is a duration), data gaps occur and the tool returns an
   error. Set `frequency <= since`.

4. **`--once` vs `run-once`**: `--once` (CLI flag) runs ALL KPIs once.
   `"run-once": true` (in kpis.json) runs only THAT specific KPI once while others
   continue at their frequency.

5. **Thanos URL format**: Pass without `https://` prefix — the tool adds it.

6. **SQLite concurrency**: SQLite is single-writer. For high-frequency collection
   across many KPIs, use `--db-type postgres`.

7. **PromQL escaping in JSON**: Backslashes in regex need double-escaping:
   `"id=~\"/system.slice/.*\""` in JSON becomes `id=~"/system.slice/.*"` in PromQL.

8. **TLS certificate errors**: Lab and disconnected clusters often use self-signed
   certs. If queries fail with `x509: certificate signed by unknown authority`,
   retry with `--insecure-tls`.

9. **Sandbox write restriction**: The default SQLite database lives at
   `~/.kpi-collector/kpi_metrics.db`, which is outside the workspace. IDE sandbox
   environments block writes to paths outside the project directory. If
   `kpi-collector run` fails with `"attempt to write a readonly database"`, tell
   the user to run the command in their own terminal instead. Read-only commands
   like `db show` and `db show errors` work fine in the sandbox.

## Workflow: End-to-End Cluster Monitoring Setup

1. Generate kpis.json based on customer requirements
2. Run collection:
   ```bash
   kpi-collector run --cluster-name customer-ran-01 --cluster-type ran \
     --kubeconfig ~/.kube/config --kpis-file kpis.json \
     --frequency 60s --duration 24h
   ```
3. Verify data is flowing:
   ```bash
   kpi-collector db show clusters
   kpi-collector db show kpis --cluster-name customer-ran-01 --limit 5
   kpi-collector db show errors
   ```
4. Launch Grafana:
   ```bash
   kpi-collector grafana start --datasource=sqlite
   ```
5. Open `http://localhost:3000` (login: admin/admin)

## Workflow: Debug a KPI Returning No Data

1. Check errors: `kpi-collector db show errors`
2. Verify the PromQL works directly against Thanos/Prometheus
3. Common fixes:
   - Widen the rate window: `[1m]` → `[5m]`
   - Check label values exist on the target cluster
   - Verify the metric name is exposed (some metrics require specific exporters)
   - For CPU placeholder queries, confirm PerformanceProfile CRs exist on the cluster
