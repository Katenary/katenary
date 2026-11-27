#!/usr/bin/env bash
# Run the Katenary end-to-end scenarios on a local kind cluster.
set -euo pipefail

E2E_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$E2E_ROOT/.." && pwd)"
export E2E_ROOT REPO_ROOT

export CACHE_DIR="${CACHE_DIR:-$REPO_ROOT/.cache/e2e}"
export KATENARY_BIN="${KATENARY_BIN:-$CACHE_DIR/katenary}"
export E2E_CLUSTER="${E2E_CLUSTER:-katenary-e2e}"
export E2E_HTTP_PORT="${E2E_HTTP_PORT:-9080}"
export E2E_HTTPS_PORT="${E2E_HTTPS_PORT:-9443}"
export E2E_KEEP="${E2E_KEEP:-0}"
export E2E_RECREATE="${E2E_RECREATE:-0}"
export TRAEFIK_REPO="${TRAEFIK_REPO:-https://traefik.github.io/charts}"
export TRAEFIK_CHART_VERSION="${TRAEFIK_CHART_VERSION:-}"
export RELEASE="${RELEASE:-rel}"

# kind auto-detects podman when docker is missing, but make it explicit for
# developers running only podman.
if [[ -z "${KIND_EXPERIMENTAL_PROVIDER:-}" ]] && ! command -v docker >/dev/null 2>&1; then
    export KIND_EXPERIMENTAL_PROVIDER=podman
fi

# shellcheck source=lib.sh
source "$E2E_ROOT/lib.sh"

usage() {
    cat <<EOF
Usage: $0 [options] [scenario ...]

Options:
  --keep      Keep the scenario namespaces for debugging (same as E2E_KEEP=1)
  --recreate  Delete and recreate the kind cluster
  --list      List the available scenarios and exit
  -h, --help  Show this help

Scenarios can be selected by name or substring, for example:
  $0 ingress
  $0 depends-on same-pod

Environment:
  E2E_HTTP_PORT=9080 E2E_HTTPS_PORT=9443   Override the default host ports
  E2E_KEEP=1                               Keep the namespaces
  E2E_RECREATE=1                           Recreate the cluster
  E2E_CLUSTER=name                         kind cluster name
  TRAEFIK_CHART_VERSION=x.y.z              Pin the Traefik chart version
EOF
}

list_scenarios() {
    find "$E2E_ROOT/scenarios" -maxdepth 1 -name '*.sh' -printf '%f\n' |
        sed 's/\.sh$//' | sort
}

selected_scenarios=()
for arg in "$@"; do
    case "$arg" in
        --keep) E2E_KEEP=1 ;;
        --recreate) E2E_RECREATE=1 ;;
        --list)
            list_scenarios
            exit 0
            ;;
        -h | --help)
            usage
            exit 0
            ;;
        -*)
            usage >&2
            die "unknown option: $arg"
            ;;
        *) selected_scenarios+=("$arg") ;;
    esac
done

for command in kind kubectl helm curl go; do
    require_command "$command"
done

# rootless port check --------------------------------------------------------
check_host_ports() {
    [[ "$(id -u)" -eq 0 ]] && return 0

    local port_start
    port_start="$(sysctl -n net.ipv4.ip_unprivileged_port_start 2>/dev/null || echo 1024)"

    local port
    for port in "$E2E_HTTP_PORT" "$E2E_HTTPS_PORT"; do
        if ((port < port_start)); then
            cat >&2 <<EOF
The host port $port cannot be bound by a rootless process:
  net.ipv4.ip_unprivileged_port_start = $port_start

The harness uses unprivileged ports by default:
  E2E_HTTP_PORT=9080 E2E_HTTPS_PORT=9443 make e2e

If you really want port 80/443, either:

Option A - allow privileged ports (temporary, until reboot):
  sudo sysctl -w net.ipv4.ip_unprivileged_port_start=80

Option A - persistent:
  echo 'net.ipv4.ip_unprivileged_port_start=80' | \\
    sudo tee /etc/sysctl.d/99-kind-katenary.conf
  sudo sysctl --system

Note: lowering the sysctl lets any local user bind ports below 1024.
EOF
            exit 1
        fi
    done
}

host_port_in_use() {
    local port="$1"
    (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null || return 1
    exec 3>&-
    return 0
}

# cluster --------------------------------------------------------------------
mkdir -p "$CACHE_DIR"

kind_config="$E2E_ROOT/kind.yaml"
if [[ "$E2E_HTTP_PORT" != 9080 || "$E2E_HTTPS_PORT" != 9443 ]]; then
    kind_config="$CACHE_DIR/kind.yaml"
    sed \
        -e "s/hostPort: 9080\$/hostPort: ${E2E_HTTP_PORT}/" \
        -e "s/hostPort: 9443\$/hostPort: ${E2E_HTTPS_PORT}/" \
        "$E2E_ROOT/kind.yaml" >"$kind_config"
fi

check_host_ports

if [[ "$E2E_RECREATE" == 1 ]]; then
    log "Deleting kind cluster $E2E_CLUSTER"
    kind delete cluster --name "$E2E_CLUSTER" >/dev/null 2>&1 || true
fi

if kind get clusters 2>/dev/null | grep -qx "$E2E_CLUSTER"; then
    log "Using existing kind cluster $E2E_CLUSTER"
else
    for port in "$E2E_HTTP_PORT" "$E2E_HTTPS_PORT"; do
        if host_port_in_use "$port"; then
            die "host port $port is already in use, stop the process that listens on it or use E2E_HTTP_PORT/E2E_HTTPS_PORT"
        fi
    done
    log "Creating kind cluster $E2E_CLUSTER"
    kind create cluster \
        --name "$E2E_CLUSTER" \
        --config "$kind_config" \
        --wait 180s
fi

kubectl config use-context "kind-$E2E_CLUSTER" >/dev/null

# traefik --------------------------------------------------------------------
log "Installing Traefik"
traefik_args=(
    traefik traefik
    --repo "$TRAEFIK_REPO"
    --namespace traefik
    --create-namespace
    --values "$E2E_ROOT/traefik-values.yaml"
    --wait
    --timeout 5m
)
if [[ -n "$TRAEFIK_CHART_VERSION" ]]; then
    traefik_args+=(--version "$TRAEFIK_CHART_VERSION")
fi
helm upgrade --install "${traefik_args[@]}"
kubectl --namespace traefik rollout status daemonset/traefik --timeout=180s

# binary ---------------------------------------------------------------------
log "Building Katenary"
mkdir -p "$CACHE_DIR/charts"
(cd "$REPO_ROOT" && go build -o "$KATENARY_BIN" ./cmd/katenary)

# scenarios ------------------------------------------------------------------
scenarios=()
while IFS= read -r scenario; do
    scenarios+=("$scenario")
done < <(find "$E2E_ROOT/scenarios" -maxdepth 1 -name '*.sh' | sort)

if [[ ${#selected_scenarios[@]} -gt 0 ]]; then
    filtered=()
    for scenario in "${scenarios[@]}"; do
        name="$(basename "$scenario" .sh)"
        for filter in "${selected_scenarios[@]}"; do
            if [[ "$name" == *"$filter"* ]]; then
                filtered+=("$scenario")
                break
            fi
        done
    done
    scenarios=("${filtered[@]}")
fi

[[ ${#scenarios[@]} -gt 0 ]] || die "no scenario matched the selection"

passed=0
failed=()

for scenario in "${scenarios[@]}"; do
    name="$(basename "$scenario" .sh)"
    export SCENARIO="$name"
    export NAMESPACE="e2e-$name"

    log "Scenario $name (namespace $NAMESPACE)"

    # A previous run may still be deleting the namespace.
    if kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
        kubectl wait --for=delete "namespace/$NAMESPACE" --timeout=120s >/dev/null 2>&1 || true
    fi

    kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml |
        kubectl apply -f - >/dev/null

    if "$scenario"; then
        printf '\033[1;32mPASS\033[0m %s\n' "$name"
        passed=$((passed + 1))
    else
        printf '\033[1;31mFAIL\033[0m %s\n' "$name"
        failed+=("$name")
        export NAMESPACE
        dump_diagnostics || true
    fi

    if [[ "$E2E_KEEP" == 1 ]]; then
        warn "keeping namespace $NAMESPACE"
    else
        kubectl delete namespace "$NAMESPACE" --wait=false >/dev/null 2>&1 || true
    fi
done

printf '\n%d scenario(s) passed, %d failed\n' "$passed" "${#failed[@]}"
if [[ ${#failed[@]} -gt 0 ]]; then
    printf 'failed: %s\n' "${failed[*]}"
    exit 1
fi
