# End-to-end tests

These tests validate Katenary against a real Kubernetes cluster. They are
**local only**: they are not part of `make test` and they are not run in CI.

The harness creates a [kind](https://kind.sigs.k8s.io/) cluster, installs
Traefik, converts the fixtures with the local Katenary binary and checks the
generated resources with `kubectl`, `helm` and `curl`.

## Prerequisites

- `podman` (recommended) or `docker`
- `kind`, `kubectl`, `helm`, `go` and `curl` in `PATH`
- a working DNS resolver for `*.127.0.0.1.nip.io` (used as ingress hostnames)

### Rootless podman and the 80/443 ports

kind maps host ports to the node. With rootless podman, the kernel refuses to
bind ports below `net.ipv4.ip_unprivileged_port_start` (usually `1024`):

```console
$ sysctl net.ipv4.ip_unprivileged_port_start
net.ipv4.ip_unprivileged_port_start = 1024
```

You have two options.

**Option A — allow privileged ports (default in `kind.yaml`).**

Temporary, until the next reboot:

```bash
sudo sysctl -w net.ipv4.ip_unprivileged_port_start=80
```

Persistent:

```bash
echo 'net.ipv4.ip_unprivileged_port_start=80' |
  sudo tee /etc/sysctl.d/99-kind-katenary.conf
sudo sysctl --system
```

This lets any local user bind ports below 1024. Use option B if you do not want
that.

**Option B — use unprivileged host ports.**

```bash
E2E_HTTP_PORT=8080 E2E_HTTPS_PORT=8443 make e2e
```

The generated URLs then use `:8080`, and `e2e/run.sh` generates a temporary
kind configuration with the right `extraPortMappings`. The Traefik entrypoints
inside the node stay on 80/443.

The runner checks the sysctl before creating the cluster and prints these
instructions when the selected ports cannot be bound.

## Usage

```bash
# Run every scenario (the cluster is reused between runs)
make e2e

# Run only matching scenarios
./e2e/run.sh depends-on
./e2e/run.sh ingress same-pod

# List the scenarios
./e2e/run.sh --list

# Keep the namespaces for debugging
./e2e/run.sh --keep same-pod

# Recreate the cluster from scratch
./e2e/run.sh --recreate

# Delete the cluster
make e2e-clean
```

Environment variables:

| Variable | Default | Description |
|---|---|---|
| `E2E_CLUSTER` | `katenary-e2e` | kind cluster name |
| `E2E_HTTP_PORT` | `80` | host port mapped to the node HTTP port |
| `E2E_HTTPS_PORT` | `443` | host port mapped to the node HTTPS port |
| `E2E_KEEP` | `0` | keep the scenario namespaces |
| `E2E_RECREATE` | `0` | delete and recreate the cluster |
| `TRAEFIK_CHART_VERSION` | latest | pin the Traefik chart version |
| `TRAEFIK_REPO` | `https://traefik.github.io/charts` | Traefik chart repository |

## Scenarios

| Scenario | What it validates |
|---|---|
| `10-ingress` | Standard Ingress and Traefik IngressRoute, `nip.io` routing, `web.ingress.enabled=false` removes the route |
| `20-depends-on` | Kubernetes API method: RBAC objects, init container waits for the dependency readiness, pod starts after it |
| `21-depends-on-legacy` | Legacy method: `busybox` + `nc` init container waits for a delayed TCP service |
| `30-same-pod` | One deployment for both services, merged containers, shared volume and PVC, merged service ports |
| `40-configmap-files` | ConfigMap generated from a root file and a nested file, `subPath` mounts, content served by nginx |

## Cluster layout

- Cluster name: `katenary-e2e`, context `kind-katenary-e2e`.
- One control-plane node with host ports 80/443 (or `E2E_HTTP_PORT` /
  `E2E_HTTPS_PORT`) mapped to the node.
- Traefik installed in the `traefik` namespace as a DaemonSet with
  `hostPort` 80/443, both Ingress and CRD providers enabled, and a default
  `IngressClass`.
- Ingress hostnames use `*.127.0.0.1.nip.io`, which resolves to `127.0.0.1`.
- Fixtures live in `e2e/fixtures/<name>/compose.yaml`.
- Scenarios are executable scripts in `e2e/scenarios/`, sourced with
  `e2e/lib.sh`.

## Debugging

When a scenario fails, the runner collects diagnostics in:

```text
.cache/e2e/diagnostics/<scenario>/
├── resources.txt
├── events.txt
├── describe.txt
└── pod-*.log
```

Use `--keep` to keep the namespaces and inspect them manually:

```bash
./e2e/run.sh --keep same-pod
kubectl --namespace e2e-30-same-pod get pods
```

The generated charts are kept in `.cache/e2e/charts/<scenario>/`.

## Limitations

- The tests pull images from Docker Hub, quay.io and the Traefik chart from the
  network on the first run.
- They are not run in CI: kind needs a privileged runner.
- Host port 443 is only used to reserve the port for Traefik; the scenarios
  currently exercise HTTP.
