#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/same-pod/compose.yaml"
chart="$CACHE_DIR/charts/$SCENARIO"

katenary_convert "$fixture" "$chart"
helm_install "$chart"

wait_pod_ready web

deployments="$(kubectl_ns get deployments -o name | grep -c . || true)"
assert_equals "1" "$deployments" "the same-pod services must produce a single deployment"

pod="$(pod_name web)"
containers="$(kubectl_ns get pod "$pod" \
    -o jsonpath='{range .spec.containers[*]}{.name}{"\n"}{end}')"
assert_contains "$containers" "web" "the web container is missing"
assert_contains "$containers" "writer" "the merged writer container is missing"

written="$(kubectl_ns exec "$pod" -c writer -- cat /shared/index.html | tr -d '\r\n')"
assert_equals "same-pod" "$written" "the shared volume was not written by the sidecar"

served="$(kubectl_ns exec "$pod" -c web -- cat /usr/share/nginx/html/index.html | tr -d '\r\n')"
assert_equals "same-pod" "$served" "the target container does not see the shared volume"

service="$(service_name web)"
ports="$(kubectl_ns get service "$service" -o jsonpath='{range .spec.ports[*]}{.port}{" "}{end}')"
assert_contains "$ports" "80" "the web port is missing from the service"
assert_contains "$ports" "9000" "the merged writer port is missing from the service"

pvc_count="$(kubectl_ns get pvc -l app.kubernetes.io/component=web -o name | grep -c . || true)"
if [[ "$pvc_count" -lt 1 ]]; then
    die "the shared PVC was not created"
fi

start_port_forward "$service" 18080
wait_http "http://127.0.0.1:18080" "same-pod" >/dev/null
