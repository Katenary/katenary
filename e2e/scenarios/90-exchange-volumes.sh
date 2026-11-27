#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/exchange-volumes/compose.yaml"
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
assert_contains "$containers" "sidecar" "the merged sidecar container is missing"

inits="$(kubectl_ns get pod "$pod" \
    -o jsonpath='{range .spec.initContainers[*]}{.name}{"\n"}{end}')"
assert_contains "$inits" "exhange-init-web" "the exchange volume init container is missing"

served="$(kubectl_ns exec "$pod" -c web -- cat /usr/share/nginx/html/index.html | tr -d '\r\n')"
assert_equals "from-exchange" "$served" "the exchange volume was not initialized for web"

shared="$(kubectl_ns exec "$pod" -c sidecar -- cat /shared/index.html | tr -d '\r\n')"
assert_equals "from-exchange" "$shared" "the sidecar does not see the exchange volume"

service="$(service_name web)"
start_port_forward "$service" 18082
wait_http "http://127.0.0.1:18082" "from-exchange" >/dev/null
