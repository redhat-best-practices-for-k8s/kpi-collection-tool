# Telco PromQL Reference for kpi-collector

Comprehensive PromQL queries for OpenShift Telco clusters, organized by cluster type.
All queries are formatted as kpis.yaml entries ready to copy. These match the built-in
profiles available via `kpi-collector kpis generate --profile <ran|core|hub>`.

## RAN Cluster

RAN clusters run DU/CU workloads with real-time requirements. Focus on CPU pinning
compliance, PTP timing, low-level system resource usage, and SRIOV networking.

### CPU & Resource Isolation

```yaml
- id: cpu-reserved-cores
  promquery: avg by (cpu) (rate(node_cpu_seconds_total{cpu=~"{{RESERVED_CPUS}}",mode!="idle"}[5m]))
  category: cpu

- id: cpu-isolated-cores
  promquery: avg by (cpu) (rate(node_cpu_seconds_total{cpu=~"{{ISOLATED_CPUS}}",mode!="idle"}[5m]))
  category: cpu

- id: cpu-node-total
  promquery: 100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
  category: cpu

- id: cpu-system-slice
  promquery: sort_desc(rate(container_cpu_usage_seconds_total{id=~"/system.slice/.*"}[5m]))
  category: cpu

- id: cpu-ovs-slice
  promquery: sort_desc(rate(container_cpu_usage_seconds_total{id=~"/ovs.slice/.*"}[5m]))
  category: cpu

- id: cpu-pods-average
  promquery: sort_desc(avg_over_time(pod:container_cpu_usage:sum[5m]))
  category: cpu
```

### Memory & Hugepages

```yaml
- id: memory-node-used-percentage
  promquery: 100 * (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes))
  category: memory

- id: memory-working-set-by-pod
  promquery: sort_desc(sum(container_memory_working_set_bytes{container!="",container!="POD"}) by (pod, namespace))
  category: memory

- id: memory-rss-by-pod
  promquery: sort_desc(sum(container_memory_rss{container!="",container!="POD"}) by (pod, namespace))
  category: memory

- id: hugepages-1g-free
  promquery: node_hugepages_free{hugepagesize="1048576"}
  category: memory

- id: hugepages-1g-total
  promquery: node_hugepages_total{hugepagesize="1048576"}
  category: memory

- id: hugepages-2m-free
  promquery: node_hugepages_free{hugepagesize="2048"}
  category: memory

- id: hugepages-2m-total
  promquery: node_hugepages_total{hugepagesize="2048"}
  category: memory
```

### PTP (Precision Time Protocol)

Requires: ptp-operator with linuxptp daemons (ptp4l, phc2sys) running.

```yaml
- id: ptp-offset-master
  promquery: openshift_ptp_offset_ns{iface="CLOCK_REALTIME",process="phc2sys"}
  category: ptp

- id: ptp-max-offset-master
  promquery: openshift_ptp_max_offset_ns{iface="CLOCK_REALTIME",process="phc2sys"}
  category: ptp

- id: ptp-clock-state
  promquery: openshift_ptp_clock_state
  category: ptp

- id: ptp-interface-role
  promquery: openshift_ptp_interface_role
  category: ptp
```

#### PTP clock state values

| Value | Meaning |
|-------|---------|
| 0 | FREERUN — not synchronized |
| 1 | LOCKED — synchronized to GM |
| 2 | HOLDOVER — lost GM, using local oscillator |

### Network

```yaml
- id: network-node-rx-bytes
  promquery: sort_desc(rate(node_network_receive_bytes_total{device!~"lo|veth.*|br.*|ovs.*"}[5m]))
  category: network

- id: network-node-tx-bytes
  promquery: sort_desc(rate(node_network_transmit_bytes_total{device!~"lo|veth.*|br.*|ovs.*"}[5m]))
  category: network

- id: network-node-rx-errors
  promquery: sort_desc(rate(node_network_receive_errs_total{device!~"lo|veth.*|br.*|ovs.*"}[5m]))
  category: network

- id: network-node-tx-errors
  promquery: sort_desc(rate(node_network_transmit_errs_total{device!~"lo|veth.*|br.*|ovs.*"}[5m]))
  category: network

- id: network-container-rx-bytes
  promquery: sort_desc(rate(container_network_receive_bytes_total{interface="eth0"}[5m]))
  category: network

- id: network-container-tx-bytes
  promquery: sort_desc(rate(container_network_transmit_bytes_total{interface="eth0"}[5m]))
  category: network
```

### OVN-Kubernetes

Requires: OVN-Kubernetes network plugin (default on OpenShift 4.x).

```yaml
- id: ovn-controller-cpu
  promquery: rate(container_cpu_usage_seconds_total{container="ovn-controller"}[5m])
  category: network

- id: ovn-controller-memory
  promquery: container_memory_working_set_bytes{container="ovn-controller"}
  category: network
```

### Disk I/O

```yaml
- id: disk-io-read-bytes
  promquery: sort_desc(rate(node_disk_read_bytes_total{device!~"dm-.*"}[5m]))
  category: disk

- id: disk-io-write-bytes
  promquery: sort_desc(rate(node_disk_written_bytes_total{device!~"dm-.*"}[5m]))
  category: disk

- id: disk-usage-percentage
  promquery: 100 - ((node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"} / node_filesystem_size_bytes{mountpoint="/",fstype!="tmpfs"}) * 100)
  category: disk
```

### Kernel & System

```yaml
- id: node-context-switches
  promquery: rate(node_context_switches_total[5m])
  category: system

- id: node-interrupts
  promquery: rate(node_intr_total[5m])
  category: system
```

### Pod Health

```yaml
- id: pod-restart-count
  promquery: sort_desc(sum(kube_pod_container_status_restarts_total) by (namespace, pod))
  category: pod-health
```

## Core Cluster

Core clusters run 5G core network functions. Focus on API server health, etcd stability,
namespace-level resource consumption, ingress performance, and storage.

### API Server

```yaml
- id: apiserver-request-duration-99p
  promquery: histogram_quantile(0.99, sum(rate(apiserver_request_duration_seconds_bucket{verb!="WATCH"}[5m])) by (verb, le))
  category: apiserver

- id: apiserver-request-rate
  promquery: sum(rate(apiserver_request_total[5m])) by (verb, code)
  category: apiserver

- id: apiserver-error-rate
  promquery: sum(rate(apiserver_request_total{code=~"5.."}[5m])) by (verb)
  category: apiserver
```

### etcd

```yaml
- id: etcd-db-size
  promquery: etcd_mvcc_db_total_size_in_bytes
  category: etcd

- id: etcd-disk-wal-fsync-duration-99p
  promquery: histogram_quantile(0.99, sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket[5m])) by (le))
  category: etcd

- id: etcd-leader-changes
  promquery: increase(etcd_server_leader_changes_seen_total[1h])
  sample-frequency: 5m
  category: etcd
```

### CPU & Memory

```yaml
- id: cpu-node-total
  promquery: 100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
  category: cpu

- id: cpu-usage-by-namespace
  promquery: sort_desc(sum(rate(container_cpu_usage_seconds_total{container!="",container!="POD"}[5m])) by (namespace))
  category: cpu

- id: memory-node-used-percentage
  promquery: 100 * (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes))
  category: memory

- id: memory-usage-by-namespace
  promquery: sort_desc(sum(container_memory_working_set_bytes{container!="",container!="POD"}) by (namespace))
  category: memory
```

### Network & Ingress

```yaml
- id: network-node-rx-bytes
  promquery: sort_desc(rate(node_network_receive_bytes_total{device!~"lo|veth.*|br.*|ovs.*"}[5m]))
  category: network

- id: network-node-tx-bytes
  promquery: sort_desc(rate(node_network_transmit_bytes_total{device!~"lo|veth.*|br.*|ovs.*"}[5m]))
  category: network

- id: ingress-request-rate
  promquery: sum(rate(haproxy_server_http_responses_total[5m])) by (code)
  category: network
```

### Disk & Storage

```yaml
- id: disk-usage-percentage
  promquery: 100 - ((node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"} / node_filesystem_size_bytes{mountpoint="/",fstype!="tmpfs"}) * 100)
  category: disk

- id: pv-usage-percentage
  promquery: 100 * (1 - (kubelet_volume_stats_available_bytes / kubelet_volume_stats_capacity_bytes))
  category: disk

- id: disk-io-read-bytes
  promquery: sort_desc(rate(node_disk_read_bytes_total{device!~"dm-.*"}[5m]))
  category: disk

- id: disk-io-write-bytes
  promquery: sort_desc(rate(node_disk_written_bytes_total{device!~"dm-.*"}[5m]))
  category: disk
```

### System & Pod Health

```yaml
- id: node-load-1min
  promquery: node_load1
  category: system

- id: node-load-5min
  promquery: node_load5
  category: system

- id: pod-restart-count
  promquery: sort_desc(sum(kube_pod_container_status_restarts_total) by (namespace, pod))
  category: pod-health

- id: pod-status-not-ready
  promquery: count(kube_pod_status_phase{phase=~"Pending|Failed"}) by (namespace, phase)
  category: pod-health

- id: cluster-uptime
  promquery: max(time() - process_start_time_seconds{job="kubelet"})
  run-once: true
  category: cluster
```

## Hub Cluster

Hub clusters manage spoke clusters via ACM/RHACM and GitOps. Focus on control plane
health, ACM operator resources, managed cluster count, policy compliance, and GitOps.

### API Server

```yaml
- id: apiserver-request-duration-99p
  promquery: histogram_quantile(0.99, sum(rate(apiserver_request_duration_seconds_bucket{verb!="WATCH"}[5m])) by (verb, le))
  category: apiserver

- id: apiserver-request-rate
  promquery: sum(rate(apiserver_request_total[5m])) by (verb, code)
  category: apiserver

- id: apiserver-error-rate
  promquery: sum(rate(apiserver_request_total{code=~"5.."}[5m])) by (verb)
  category: apiserver
```

### etcd

```yaml
- id: etcd-db-size
  promquery: etcd_mvcc_db_total_size_in_bytes
  category: etcd

- id: etcd-disk-wal-fsync-duration-99p
  promquery: histogram_quantile(0.99, sum(rate(etcd_disk_wal_fsync_duration_seconds_bucket[5m])) by (le))
  category: etcd

- id: etcd-leader-changes
  promquery: increase(etcd_server_leader_changes_seen_total[1h])
  sample-frequency: 5m
  category: etcd
```

### CPU & Memory

```yaml
- id: cpu-node-total
  promquery: 100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
  category: cpu

- id: memory-node-used-percentage
  promquery: 100 * (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes))
  category: memory
```

### ACM (Advanced Cluster Management)

Requires: ACM operator installed (open-cluster-management, multicluster-engine namespaces).

```yaml
- id: acm-cpu-usage
  promquery: sort_desc(sum(rate(container_cpu_usage_seconds_total{container!="",container!="POD",namespace=~"open-cluster-management.*|multicluster-engine"}[5m])) by (namespace, pod))
  category: acm

- id: acm-memory-usage
  promquery: sort_desc(sum(container_memory_working_set_bytes{container!="",container!="POD",namespace=~"open-cluster-management.*|multicluster-engine"}) by (namespace, pod))
  category: acm

- id: acm-managed-clusters
  promquery: count(acm_managed_cluster_info)
  sample-frequency: 5m
  category: acm

- id: acm-policy-noncompliant
  promquery: count(policy_governance_info{type="root",compliant="NonCompliant"})
  sample-frequency: 5m
  category: acm
```

### GitOps

Requires: OpenShift GitOps operator installed (openshift-gitops namespace).

```yaml
- id: gitops-cpu-usage
  promquery: sort_desc(sum(rate(container_cpu_usage_seconds_total{container!="",container!="POD",namespace=~"openshift-gitops.*"}[5m])) by (namespace, pod))
  category: gitops

- id: gitops-memory-usage
  promquery: sort_desc(sum(container_memory_working_set_bytes{container!="",container!="POD",namespace=~"openshift-gitops.*"}) by (namespace, pod))
  category: gitops
```

### Disk, Network & System

```yaml
- id: disk-usage-percentage
  promquery: 100 - ((node_filesystem_avail_bytes{mountpoint="/",fstype!="tmpfs"} / node_filesystem_size_bytes{mountpoint="/",fstype!="tmpfs"}) * 100)
  category: disk

- id: pv-usage-percentage
  promquery: 100 * (1 - (kubelet_volume_stats_available_bytes / kubelet_volume_stats_capacity_bytes))
  category: disk

- id: network-node-rx-bytes
  promquery: sort_desc(rate(node_network_receive_bytes_total{device!~"lo|veth.*|br.*|ovs.*"}[5m]))
  category: network

- id: network-node-tx-bytes
  promquery: sort_desc(rate(node_network_transmit_bytes_total{device!~"lo|veth.*|br.*|ovs.*"}[5m]))
  category: network

- id: node-load-1min
  promquery: node_load1
  category: system
```

### Pod Health & Cluster Info

```yaml
- id: pod-restart-count
  promquery: sort_desc(sum(kube_pod_container_status_restarts_total) by (namespace, pod))
  category: pod-health

- id: pod-status-not-ready
  promquery: count(kube_pod_status_phase{phase=~"Pending|Failed"}) by (namespace, phase)
  category: pod-health

- id: cluster-uptime
  promquery: max(time() - process_start_time_seconds{job="kubelet"})
  run-once: true
  category: cluster
```

## Range Query Examples

Use range queries when you need historical data with specific resolution.
These can be added to any cluster type.

```yaml
- id: cpu-trend-1h
  promquery: avg by (instance) (rate(node_cpu_seconds_total{mode!="idle"}[5m]))
  query-type: range
  range:
    step: 30s
    since: 1h
  run-once: true

- id: memory-trend-6h
  promquery: node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes
  query-type: range
  range:
    step: 1m
    since: 6h
  sample-frequency: 6h

- id: ptp-offset-trend-24h
  promquery: abs(openshift_ptp_offset_ns)
  query-type: range
  range:
    step: 30s
    since: 24h
  run-once: true
```

## PromQL Tips for Thanos

1. **Use wider rate windows** — `[5m]` minimum. Thanos deduplication can cause gaps
   with `[1m]` or `[2m]` windows.

2. **Prefer `avg_over_time` for smoothing** — when querying through Thanos,
   `avg_over_time(metric[5m])` produces smoother results than raw instant queries.

3. **Label filtering matters** — always filter `container!=""` in container metrics
   to exclude the pause container aggregation.

4. **`topk` and `sort_desc`** — use `topk(N, ...)` to limit cardinality for
   high-cardinality metrics. Use `sort_desc(...)` when you want all values ordered.

5. **Regex for multi-value labels** — use `cpu=~"0|1|2|3"` for specific CPUs.
   The `{{RESERVED_CPUS}}` placeholder does this automatically.

6. **Absent metrics** — use `absent(metric_name)` to detect when an exporter is
   down or a metric is not being scraped.
