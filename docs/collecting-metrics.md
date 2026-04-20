# Collecting Metrics

The `run` command gathers KPI metrics from Prometheus/Thanos and stores them in a database.

> [!TIP]
> **New here?** Start with the [Getting Started](getting-started.md) tutorial to collect your first metrics in 5 minutes.

Related guides:

- [Getting Started](getting-started.md)
- [KPI Configuration](kpis-file-configuration.md)
- [Database Commands](database-commands.md)
- [Grafana](grafana.md)

## Authentication Modes

kpi-collector supports two authentication modes.


| Mode                       | When to use                                                                                                                                                                                                                                                                                         | Requirements                                                                                    |
| -------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `--kubeconfig`             | You have kubeconfig access to the cluster. The tool auto-discovers the Thanos URL, creates a short-lived bearer token, and can fetch and template [CPU IDs from PerformanceProfile CRs](#dynamic-cpu-ids-from-performanceprofile-crs).                                                                                 | Admin privileges or permissions to create tokens for `prometheus-k8s` in `openshift-monitoring` |
| `--token` + `--thanos-url` | You don't have kubeconfig access, or you want to use a specific token/URL. You provide both values manually. Automatic [CPU ID resolution](#dynamic-cpu-ids-from-performanceprofile-crs) is not available — see the [manual alternative](#manual-alternative-obtaining-cpu-ids-without---kubeconfig) to hardcode them. | A valid bearer token and the Thanos querier hostname                                            |


### 1) Using Kubeconfig (Automatic Discovery)

```bash
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --kubeconfig ~/.kube/config \
  --kpis-file kpis.json
```

#### What happens behind the scenes

When you pass `--kubeconfig`, the tool performs two Kubernetes API calls before any metrics are collected:

1. **Discovers the Thanos querier URL** — reads the OpenShift route `thanos-querier` in the `openshift-monitoring` namespace and extracts the hostname from `route.spec.host`.
2. **Creates a short-lived bearer token** — requests a token for the `prometheus-k8s` service account in `openshift-monitoring`. The token expiry is set automatically to match the collection duration plus a 10-minute buffer, so it won't expire mid-run. When `--once` is used the token is short-lived (10 minutes).

These are equivalent to:

```bash
# Discover Thanos URL
oc get route thanos-querier -n openshift-monitoring -o jsonpath='{.spec.host}'

# Create token — for a 45m collection (default): duration + 10m buffer = 55m
oc create token prometheus-k8s -n openshift-monitoring --duration=55m

# Create token — for --once mode: fixed 10m
oc create token prometheus-k8s -n openshift-monitoring --duration=10m
```

No new service account is created; the tool uses the existing `prometheus-k8s` account that ships with OpenShift monitoring.

After these two steps, the discovered URL and token are used exactly like the manual `--token` / `--thanos-url` flags for the rest of the collection run.

#### How metrics are stored

Each query returns a Prometheus result — either a **vector** (instant queries) or a **matrix** (range queries). The tool parses the result, extracts each individual sample, and stores it as a separate row in the database with its value, timestamp, and labels (serialized as JSON). You can then query the stored data with [`db show`](database-commands.md) or visualize it in [Grafana](grafana.md).

#### Additional examples

With custom sampling parameters:

```bash
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --kubeconfig ~/.kube/config \
  --kpis-file kpis.json \
  --frequency 30s \
  --duration 1h
```

Using PostgreSQL:

```bash
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --kubeconfig ~/.kube/config \
  --kpis-file kpis.json \
  --db-type postgres \
  --postgres-url "postgresql://myuser:mypass@localhost:5432/kpi_metrics?sslmode=disable"
```

### 2) Using Token and Thanos URL (Manual)

Use this mode when you don't have kubeconfig access, or when the `--kubeconfig` auto-discovery isn't viable (e.g. non-standard monitoring namespace, bastion host access, or you obtained credentials through a different flow).

#### Step-by-step: obtaining the values from an OpenShift cluster

You need `oc` CLI access to the cluster (or ask your cluster admin for these values).

**1. Get the Thanos querier URL**

```bash
oc get route thanos-querier -n openshift-monitoring -o jsonpath='{.spec.host}'
```

Example output: `thanos-querier-openshift-monitoring.apps.mycluster.example.com`

Export it for convenience:

```bash
export THANOS_URL=$(oc get route thanos-querier -n openshift-monitoring \
  -o jsonpath='{.spec.host}')
```

> [!TIP]
> If your cluster uses a custom monitoring namespace or a different
> route name, adjust the namespace (`-n`) and route name accordingly.

**2. Create a bearer token**

```bash
oc create token prometheus-k8s -n openshift-monitoring --duration=55m
```

The `prometheus-k8s` service account ships with OpenShift monitoring and has the permissions required to query Thanos. The `--duration` flag controls how long the token stays valid — set it to at least your collection `--duration` plus some buffer (the tool adds 10 minutes automatically when using `--kubeconfig`).

Export it:

```bash
export TOKEN=$(oc create token prometheus-k8s -n openshift-monitoring --duration=55m)
```

> [!IMPORTANT]
> If you need to use a different service account, make sure it has
> `get` permissions on the Prometheus/Thanos API. Create the token from that
> service account instead.

**3. Run kpi-collector**

```bash
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --token $TOKEN \
  --thanos-url $THANOS_URL \
  --kpis-file kpis.json
```

## `--insecure-tls`

Use this flag when running against clusters or Prometheus/Thanos servers with self-signed or untrusted certificates.

```bash
kpi-collector run \
  --cluster-name my-cluster \
  --cluster-type ran \
  --kubeconfig ~/.kube/config \
  --kpis-file kpis.json \
  --insecure-tls
```

What it does:

- Skips TLS certificate verification for HTTPS requests
- Applies to Kubernetes API calls and Thanos/Prometheus queries
- Helps in environments where the certificate cannot be validated

When to use:

- Self-signed certificates
- Disconnected, lab, or air-gapped clusters
- kubeconfig without a valid CA
- Errors such as `x509: certificate signed by unknown authority`

Complete example:

```bash
kpi-collector run \
  --cluster-name dev-cluster \
  --cluster-type ran \
  --kubeconfig ~/.kube/config \
  --kpis-file kpis.json \
  --frequency 60s \
  --duration 1h \
  --insecure-tls
```

## Command Line Flags (`run`)


| Flag              | Required | Default                      | Description                                                             |
| ----------------- | -------- | ---------------------------- | ----------------------------------------------------------------------- |
| `--cluster-name`  | Yes      | -                            | Name of the cluster being monitored                                     |
| `--cluster-type`  | Yes      | -                            | Cluster type for categorization: `ran`, `core`, or `hub`                |
| `--kubeconfig`    | No*      | -                            | Path to kubeconfig file for auto-discovery                              |
| `--token`         | No*      | -                            | Bearer token for Thanos authentication                                  |
| `--thanos-url`    | No*      | -                            | Thanos querier URL (without `https://`)                                 |
| `--insecure-tls`  | No       | false                        | Skip TLS certificate verification (dev only)                            |
| `--frequency`     | No       | 1m                           | Sampling frequency (for example: `10s`, `1m`, `2h`, `24h`)              |
| `--duration`      | No       | 45m                          | Total sampling duration (for example: `10s`, `1m`, `2h`, `24h`)         |
| `--db-type`       | No       | sqlite                       | Database type: `sqlite` or `postgres`                                   |
| `--postgres-url`  | No**     | -                            | PostgreSQL connection string                                            |
| `--once`          | No       | false                        | Collect all KPIs once and exit (ignores `--frequency` and `--duration`) |
| `--kpis-file`     | Yes      | -                            | Path to KPIs configuration file (see `kpis.json.template`)              |
| `--artifacts-dir` | No       | `./kpi-collector-artifacts/` | Directory for database, logs, and output files                          |


 Either provide `--kubeconfig` OR both `--token` and `--thanos-url`  
 Required when `--db-type=postgres`

## Dynamic CPU IDs from PerformanceProfile CRs

Queries can use `{{RESERVED_CPUS}}` and `{{ISOLATED_CPUS}}` placeholders. At startup, these are replaced with actual CPU IDs from the cluster's PerformanceProfile CRs so that your PromQL queries target the correct cores. This feature requires `--kubeconfig` authentication.

Example query in `kpis.json`:

```json
{
    "id": "cpu-reserved-set",
    "promquery": "rate(node_cpu_seconds_total{cpu=~\"{{RESERVED_CPUS}}\"}[30m])"
}
```

If the cluster's PerformanceProfile defines `reserved: "0-1,32-33"`, the query
sent to Thanos becomes:

```
rate(node_cpu_seconds_total{cpu=~"0|1|32|33"}[30m])
```

### What happens behind the scenes

Before collection starts, the tool checks if any query contains `{{RESERVED_CPUS}}` or `{{ISOLATED_CPUS}}`. If so, it:

1. **Fetches all PerformanceProfile CRs** from the cluster via the `performance.openshift.io/v2` API (equivalent to `oc get performanceprofiles -o json`).
2. **Reads `spec.cpu.reserved` and `spec.cpu.isolated`** from each profile. If multiple PerformanceProfiles exist, the CPU sets from all profiles are aggregated (union of all CPU IDs).
3. **Converts CPU ranges to Prometheus regex format** — range notation like `"0-3,8-11"` is expanded to individual CPU IDs and joined with `|` to produce `"0|1|2|3|8|9|10|11"`, ready for use in PromQL `=~` matchers.
4. **Substitutes placeholders** in every query that contains them.

### Manual alternative: obtaining CPU IDs without --kubeconfig

If `--kubeconfig` is not available, you can fetch the CPU sets manually and hardcode them in your `kpis.json`.

**1. List PerformanceProfiles**

```bash
oc get performanceprofiles
```

**2. Get the CPU sets from a specific profile**

```bash
# Reserved CPUs
oc get performanceprofile <profile-name> -o jsonpath='{.spec.cpu.reserved}'
# Example output: 0-1,32-33

# Isolated CPUs
oc get performanceprofile <profile-name> -o jsonpath='{.spec.cpu.isolated}'
# Example output: 2-31,34-63
```

**3. Convert to Prometheus regex format**

Expand the ranges and join with `|`:

- `0-1,32-33` → `0|1|32|33`
- `2-31,34-63` → `2|3|4|...|31|34|35|...|63`

**4. Use the values directly in your query**

```json
{
    "id": "cpu-reserved-set",
    "promquery": "rate(node_cpu_seconds_total{cpu=~\"0|1|32|33\"}[30m])"
}
```

This approach avoids the need for `--kubeconfig` at the cost of hardcoding cluster-specific CPU assignments.

## Sampling, KPI File Format, and Run Modes

For details on frequency/duration, single run mode (`--once`), per-query `run-once`, range queries, and the KPI JSON file format, see [KPI Configuration](kpis-file-configuration.md).