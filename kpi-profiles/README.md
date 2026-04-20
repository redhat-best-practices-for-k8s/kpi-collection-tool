# KPI Profiles

Ready-to-use KPI configuration files for common use cases. Pick the file that
matches your cluster profile and pass it to the collector:

```bash
kpi-collector run --kpis-file <path-to>/kpis-ran.json ...
```

## Available Profiles

| File | Use Case | KPIs |
|------|----------|------|
| `kpis-quickstart.json` | Smoke test — verify the tool connects and collects | 2 |
| `kpis-basic.json` | General cluster health (CPU, memory, disk, pods) | 11 |
| `kpis-ran.json` | RAN DU single-node clusters (reserved/isolated CPUs, hugepages, PTP, OVN) | 31 |
| `kpis-core.json` | Core clusters (control plane, etcd, API server, ingress, storage) | 22 |
| `kpis-hub.json` | Hub/ACM clusters (managed clusters, policy compliance, GitOps, etcd) | 22 |

## Profile Details

### kpis-quickstart.json

Two KPIs that return data on virtually any OpenShift or Kubernetes cluster.
Use this to confirm connectivity and database storage before moving to a
full profile.

### kpis-basic.json

A profile-agnostic health baseline: node CPU, memory, load, disk, namespace
resource consumption, pod restarts, and cluster uptime. Works on any cluster
type.

### kpis-ran.json

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

### kpis-core.json

Designed for centralized core clusters that run control-plane services,
ingress, and shared workloads. Covers:

- **Control plane** — API server latency (p99), request rate, error rate
- **etcd** — database size, WAL fsync latency, leader changes
- **Ingress** — HAProxy response rates
- **Storage** — disk usage, PV capacity, disk I/O throughput
- **Node health** — CPU, memory, load averages
- **Pod health** — restart counts, non-ready pods

### kpis-hub.json

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

Use `kpi-collector kpis generate` to create a tailored `kpis.json` from any of
the three main profiles (`ran`, `core`, `hub`):

```bash
# Generate all RAN KPIs at once
kpi-collector kpis generate --profile ran --all

# Interactively pick which categories to include
kpi-collector kpis generate --profile core

# Write to a custom path
kpi-collector kpis generate --profile hub --all -f /path/to/hub-kpis.json

# Overwrite an existing file
kpi-collector kpis generate --profile ran --all --overwrite
```

By default the output file is named `<profile>-kpis.json` in the current
directory. If the file already exists, the command will refuse to overwrite it
unless you pass `--overwrite`. In interactive mode (the default), you are
prompted per category — use `--all` to skip prompts and include everything.

Once generated, you can edit the file to fine-tune individual KPIs before
passing it to the collector.

## Building Your Own

For guidance on writing a custom KPI configuration file from scratch — including
per-KPI frequency overrides, range queries, `run-once` mode, and CPU
placeholders — see the annotated [`kpis.json.template`](../kpis.json.template)
in the project root.
