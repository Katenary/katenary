#!/usr/bin/env bash
# shellcheck shell=bash
#
# Shared helpers for the end-to-end scenarios. This file is sourced by run.sh
# and by every scenario.

E2E_ROOT="${E2E_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
REPO_ROOT="${REPO_ROOT:-$(cd "$E2E_ROOT/.." && pwd)}"
CACHE_DIR="${CACHE_DIR:-$REPO_ROOT/.cache/e2e}"
KATENARY_BIN="${KATENARY_BIN:-$CACHE_DIR/katenary}"

: "${E2E_HTTP_PORT:=80}"
: "${E2E_HTTPS_PORT:=443}"

RELEASE="${RELEASE:-rel}"

log() { printf '\n\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mWARN:\033[0m %s\n' "$*" >&2; }
die() {
    printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

kubectl_ns() { kubectl --namespace "$NAMESPACE" "$@"; }

katenary_convert() {
    local fixture="$1" chart="$2"
    rm -rf "$chart"
    "$KATENARY_BIN" convert --force --compose-file "$fixture" --output-dir "$chart"
}

helm_install() {
    local chart="$1"
    shift

    helm upgrade --install "$RELEASE" "$chart" \
        --namespace "$NAMESPACE" \
        --wait --timeout 5m \
        "$@"
}

assert_equals() {
    local expected="$1" actual="$2" message="$3"
    if [[ "$expected" != "$actual" ]]; then
        die "$message: expected '$expected', got '$actual'"
    fi
}

assert_contains() {
    local haystack="$1" needle="$2" message="$3"
    if [[ "$haystack" != *"$needle"* ]]; then
        die "$message: '$needle' not found in '$haystack'"
    fi
}

wait_http() {
    local url="$1" expected="${2:-}" tries="${3:-60}" body
    for ((i = 1; i <= tries; i++)); do
        if body="$(curl -fsS --max-time 5 "$url" 2>/dev/null)" &&
            [[ -z "$expected" || "$body" == *"$expected"* ]]; then
            printf '%s' "$body"
            return 0
        fi
        sleep 2
    done
    die "timeout waiting for $url"
}

wait_http_status() {
    local url="$1" expected="$2" tries="${3:-60}" code
    for ((i = 1; i <= tries; i++)); do
        code="$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$url" || true)"
        if [[ "$code" == "$expected" ]]; then
            return 0
        fi
        sleep 2
    done
    die "$url did not return HTTP $expected (last status: ${code:-none})"
}

wait_deployment() {
    kubectl_ns rollout status "deployment/$1" --timeout=180s >/dev/null
}

wait_pod_ready() {
    local component="$1"
    kubectl_ns wait \
        --for=condition=Ready \
        --selector="app.kubernetes.io/component=$component" \
        pod --timeout=180s >/dev/null
}

pod_name() {
    local component="$1"
    kubectl_ns get pods \
        --selector="app.kubernetes.io/component=$component" \
        -o jsonpath='{.items[0].metadata.name}'
}

service_name() {
    local component="$1"
    kubectl_ns get services \
        --selector="app.kubernetes.io/component=$component" \
        -o jsonpath='{.items[0].metadata.name}'
}

# start_port_forward <service-name> <host-port>
# Sets PORT_FORWARD_PID. The caller must call stop_port_forward (or rely on the
# EXIT trap installed by start_port_forward).
start_port_forward() {
    local service="$1" port="$2"
    kubectl_ns port-forward "service/$service" "${port}:80" \
        >"$CACHE_DIR/port-forward-$SCENARIO.log" 2>&1 &
    PORT_FORWARD_PID=$!
    trap stop_port_forward EXIT
}

stop_port_forward() {
    if [[ -n "${PORT_FORWARD_PID:-}" ]]; then
        kill "$PORT_FORWARD_PID" 2>/dev/null || true
        PORT_FORWARD_PID=""
    fi
}

# rfc3339_epoch <timestamp> prints the timestamp as a unix epoch.
rfc3339_epoch() {
    date -d "$1" +%s
}

dump_diagnostics() {
    local dir="$CACHE_DIR/diagnostics/$SCENARIO"
    mkdir -p "$dir"

    kubectl_ns get all,pvc,configmap,secret,ingress,serviceaccount,role,rolebinding \
        -o wide >"$dir/resources.txt" 2>&1 || true
    kubectl_ns get events --sort-by=.lastTimestamp >"$dir/events.txt" 2>&1 || true

    local kind
    for kind in all pvc configmap secret ingress ingressroute serviceaccount role rolebinding; do
        kubectl_ns describe "$kind" >>"$dir/describe.txt" 2>&1 || true
    done

    local pod
    while read -r pod; do
        [[ -z "$pod" ]] && continue
        local file="$dir/${pod//\//_}.log"
        kubectl_ns logs "$pod" --all-containers --prefix >"$file" 2>&1 || true
        kubectl_ns logs "$pod" --all-containers --prefix --previous >>"$file" 2>&1 || true
    done < <(kubectl_ns get pods -o name)

    warn "diagnostics written to $dir"
}
