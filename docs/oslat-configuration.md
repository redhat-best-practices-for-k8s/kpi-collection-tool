# Oslat Configuration

This guide covers the oslat YAML configuration format — all available fields,
how the pod lifecycle works, and how to write your own configuration file.

Related guides:
- [Getting Started](getting-started.md)
- [Tasks Configuration](tasks-configuration.md) — orchestrating multiple tasks
- [Task Profiles](../task-profiles/README.md) — ready-to-use quickstart and full examples

## What it does

The oslat task is a thin **orchestrator** step: it creates a user-supplied Pod,
waits until it completes (or the timeout expires), then writes the pod's logs
to an artifact file. It does **not** ship an oslat binary or replace Red Hat’s
supported latency-test tooling — you supply the Pod.

This task is optional. You can collect Prometheus metrics without it, or run
supported latency tests outside kpi-collector. Use it when you want those
results collected in the same run as other validation tasks.

Examples in this guide and under `task-profiles/` show the YAML shape and a
plumbing smoke test — do not treat them as a production latency-test Pod. For
OpenShift platform verification, use the image and Pod fields from Red Hat’s
[platform verification latency tests](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/scalability_and_performance/cnf-performing-platform-verification-latency-tests)
docs (or a config provided by your Red Hat engagement), including
`runtimeClassName` and low-latency CRIO annotations where required.

**Artifact:** `oslat_logs.out` under `--artifacts-dir`

## Configuration format

The configuration lives under an `oslat:` root key — either inline in a
`tasks.yaml` or in a standalone file referenced via `configFile`.

### Minimal example

Plumbing smoke test (not a latency measurement) — useful to verify create →
wait → collect:

```yaml
oslat:
  timeout: 5m
  pod:
    metadata:
      name: oslat-test
    spec:
      restartPolicy: Never
      containers:
        - name: oslat
          image: registry.access.redhat.com/ubi9/ubi-minimal:latest
          command: ["sh", "-c", "echo hello; sleep 15"]
```

### Full example with all fields

Schema example with common PodSpec fields. Replace the Pod contents with your
real workload (for platform verification, prefer the Red Hat docs linked above):

```yaml
oslat:
  timeout: 30m
  pod:
    metadata:
      name: oslat-validation
      namespace: latency-testing
    spec:
      restartPolicy: Never
      nodeSelector:
        kubernetes.io/hostname: worker-0.lab.example.com
      containers:
        - name: oslat
          image: your-registry.example.com/oslat:latest
          command: ["oslat"]
          args: ["-D", "1800"]
          resources:
            requests:
              cpu: "4"
              memory: 1Gi
            limits:
              cpu: "4"
              memory: 1Gi
```

## Field reference

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `timeout` | Yes | — | Maximum time to wait for the pod to complete. If the pod is still running when the timeout expires, the task collects whatever logs are available and moves on. Accepts a Go duration string (e.g. `30m`, `1h`). |
| `mountKubeconfig` | No | `false` | When `true`, the tool creates a Secret from the run's `--kubeconfig` and injects it (volume + read-only mount + `KUBECONFIG` env) into every container. Removes the need to pre-create a Secret or hand-write the volume. |
| `mountPath` | No | `/kubeconfig` | Mount path for the injected kubeconfig (only used with `mountKubeconfig`). The file lands at `<mountPath>/kubeconfig` and `KUBECONFIG` is set to it. |
| `pod` | Yes | — | A full Kubernetes Pod manifest (metadata + spec). This is the exact Pod that will be created on the cluster. |
| `pod.metadata.name` | Yes | — | The Pod name. |
| `pod.metadata.namespace` | No | cluster default | Namespace to create the pod in. |
| `pod.spec.containers[].image` | Yes | — | Container image for each container. |
| `pod.spec` | Yes | — | Standard Kubernetes PodSpec — you can use any valid fields (`runtimeClassName`, annotations, `nodeSelector`, `resources`, `tolerations`, `volumes`, etc.). |

### Validation rules

- `timeout` must be greater than 0.
- `pod` must be provided.
- `pod.metadata.name` must not be empty.
- `pod.spec.containers` must have at least one container.
- Every container must have a non-empty `image`.

## Using with a tasks file

### Inline

```yaml
# tasks.yaml
oslat:
  timeout: 30m
  pod:
    metadata:
      name: oslat-test
    spec:
      restartPolicy: Never
      containers:
        - name: oslat
          image: your-registry/oslat:latest
          command: ["oslat"]
          args: ["-D", "1800"]
```

### External file via configFile

```yaml
# tasks.yaml
oslat:
  configFile: oslat-config.yaml
```

The external file must wrap the configuration under the `oslat:` root key:

```yaml
# oslat-config.yaml
oslat:
  timeout: 30m
  pod:
    metadata:
      name: oslat-test
    spec:
      restartPolicy: Never
      containers:
        - name: oslat
          image: your-registry/oslat:latest
          command: ["oslat"]
          args: ["-D", "1800"]
```

`configFile` and inline fields are mutually exclusive — set one or the other.
Paths are resolved relative to the directory containing the tasks file.

## Pod lifecycle

1. **Create** — the task creates the Pod on the cluster exactly as specified.
2. **Wait** — the task polls the pod status until it reaches `Succeeded` or
   `Failed`, or until the `timeout` expires.
3. **Collect** — pod logs are written to `oslat_logs.out`.
4. **Cleanup** — the pod stays on the cluster so you can inspect it. Set
   `orchestration.cleanup: true` to delete it when the task finishes or fails.

## Pod spec tips

**`restartPolicy: Never`** is strongly recommended. If the pod restarts, the
task sees a running pod and continues waiting, which may cause unexpected
timeout behavior.

**`nodeSelector`** lets you pin the pod to a specific node — useful when testing
latency on a tuned worker.

**`resources`** — set CPU and memory requests/limits to ensure your latency
workload gets dedicated resources and isn't throttled.

**`tolerations`** — if your target node has taints (common on RAN/DU workers),
add matching tolerations to the pod spec.

**Low-latency fields** — for meaningful platform verification results, include
fields such as `runtimeClassName` and low-latency CRIO annotations as required
by the Red Hat latency-test docs linked above. The task creates the Pod exactly
as you specify; it does not add those fields for you.

## Authentication

Requires `--kubeconfig` with admin privileges. The task creates a pod and reads
its logs via the Kubernetes API.

### Mounting a kubeconfig into the pod

Some workloads (e.g. the Red Hat `cnf-tests` oslat harness) act as a client that
talks to the cluster API from *inside* the pod, so they need a kubeconfig mounted
in. Instead of running `oc create secret generic ... --from-file=kubeconfig=...`
and wiring up the volume by hand, set `mountKubeconfig: true`:

```yaml
oslat:
  timeout: 20m
  mountKubeconfig: true        # tool creates the Secret and injects it
  # mountPath: /kubeconfig     # optional; KUBECONFIG becomes <mountPath>/kubeconfig
  pod:
    metadata:
      name: oslat-test
    spec:
      restartPolicy: Never
      containers:
        - name: oslat
          image: your-registry/cnf-tests-rhel9:v4.22
          command: ["/usr/bin/test-run.sh"]
          args: ["--ginkgo.v", "--ginkgo.focus=oslat"]
```

The tool creates a Secret named `<pod-name>-kubeconfig` from the run's
`--kubeconfig`, mounts it read-only at `mountPath`, and sets `KUBECONFIG` on every
container (an existing `KUBECONFIG` env is left untouched).

The Secret is deleted when the task exits, whatever the pod's outcome — success,
failure, or a timeout — as long as cleanup is enabled (see
`orchestration.cleanup` in the [tasks configuration](tasks-configuration.md)).
With cleanup disabled, both the pod and the Secret are left in the cluster for
inspection.

> **Proxy note:** the mounted kubeconfig runs in-cluster, so it must **not**
> contain a `proxy-url` pointing at a laptop-local proxy (it is unreachable from
> the pod). Keep the kubeconfig proxy-free and pass the proxy to the tool with
> `--proxy-url` instead.
