# Prometheus KPI Profiles

Ready-to-use Prometheus KPI configuration files for the **prometheus** task
type. Pick the file that matches your cluster profile and reference it from
your `tasks.yaml` or pass it directly:

```yaml
# In a tasks.yaml file:
prometheus:
  configFile: ../prom-kpi-profiles/kpis-ran.yaml
```

```bash
# Or Prometheus-only mode (no tasks file):
kpi-collector run --prom-kpis-config prom-kpi-profiles/kpis-ran.yaml ...
```

## Available Profiles

| File | Use Case | KPIs |
|------|----------|------|
| `kpis-quickstart.yaml` | Smoke test — verify the tool connects and collects | 2 |
| `kpis-basic.yaml` | General cluster health (CPU, memory, disk, pods) | 11 |
| `kpis-ran.yaml` | RAN DU single-node clusters (reserved/isolated CPUs, hugepages, PTP, OVN) | 31 |
| `kpis-core.yaml` | Core clusters (control plane, etcd, API server, ingress, storage) | 22 |
| `kpis-hub.yaml` | Hub/ACM clusters (managed clusters, policy compliance, GitOps, etcd) | 22 |

## Profile Details

### kpis-quickstart.yaml

Two KPIs that return data on virtually any OpenShift or Kubernetes cluster.
Use this to confirm connectivity and database storage before moving to a
full profile.

### kpis-basic.yaml

A profile-agnostic health baseline: node CPU, memory, load, disk, namespace
resource consumption, pod restarts, and cluster uptime. Works on any cluster
type.

### kpis-ran.yaml

Tailored for RAN Distributed Unit (DU) nodes running on Single Node OpenShift
with a PerformanceProfile. Covers:

- **CPU partitioning** — reserved vs. isolated core utilization via
  `{{RESERVED_CPUS}}` and `{{ISOLATED_CPUS}}` placeholders (requires
  `--kubeconfig` so the tool can read the PerformanceProfile CR)
- **System slices** — `system.slice` and `ovs.slice` CPU consumption
- **HugePages** — 1 GiB and 2 MiB allocation tracking
- **PTP** — clock offset, max offset, clock state, interface role
- **Networking** — node and container RX/TX bytes and errors
- **OVN** — controller CPU and memory
- **System jitter** — context switches and hardware interrupts

### kpis-core.yaml

Designed for centralized core clusters that run control-plane services,
ingress, and shared workloads. Covers:

- **Control plane** — API server latency (p99), request rate, error rate
- **etcd** — database size, WAL fsync latency, leader changes
- **Ingress** — HAProxy response rates
- **Storage** — disk usage, PV capacity, disk I/O throughput
- **Node health** — CPU, memory, load averages
- **Pod health** — restart counts, non-ready pods

### kpis-hub.yaml

Built for hub clusters running Red Hat Advanced Cluster Management (ACM) and
OpenShift GitOps. Covers:

- **ACM** — managed cluster count, non-compliant policy count, per-pod
  resource usage in `open-cluster-management` and `multicluster-engine`
  namespaces
- **GitOps** — per-pod resource usage in `openshift-gitops` namespaces
- **Control plane** — API server and etcd health (same as core, since the
  hub's API load scales with managed cluster count)
- **Infrastructure** — disk, PV, network, node load, pod health

## Generating a KPI File

Use `kpi-collector kpis generate` to create an inline `tasks.yaml` from any of
the three main profiles (`ran`, `core`, `hub`). The command **always prompts
for tasks**. `--all` skips KPI **domain** prompts only — it does not skip task
prompts.

```bash
# Prompts for tasks, then includes all KPI domains for the profile
kpi-collector kpis generate --profile ran --all

# Prompts for tasks and KPI domains
kpi-collector kpis generate --profile core

# Custom output path
kpi-collector kpis generate --profile hub --all -f /path/to/hub-tasks.yaml

# Overwrite an existing file
kpi-collector kpis generate --profile ran --all --overwrite

# Generate without category fields (single-table storage)
kpi-collector kpis generate --profile ran --all --uncategorized
```

Default output is `tasks.yaml`. If the file already exists, pass `--overwrite`.
See [KPI file configuration](../docs/kpis-file-configuration.md).

Generated KPIs include a `category` field by default. Pass `--uncategorized`
to omit it and store all metrics in a single table.

## Building Your Own

For guidance on writing a custom KPI configuration file from scratch — including
per-KPI frequency overrides, range queries, `run-once` mode, and CPU
placeholders — see the annotated [`kpis.yaml.template`](../kpis.yaml.template)
in the project root.
