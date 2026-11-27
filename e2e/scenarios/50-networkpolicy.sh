#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/networkpolicy/compose.yaml"
chart="$CACHE_DIR/charts/$SCENARIO"

katenary_convert "$fixture" "$chart"
helm_install "$chart"

wait_pod_ready db
wait_pod_ready web

for component in db web; do
    count="$(kubectl_ns get networkpolicy -l "app.kubernetes.io/component=$component" -o name | grep -c . || true)"
    if [[ "$count" -lt 1 ]]; then
        die "missing NetworkPolicy for $component"
    fi
done

web_pod="$(pod_name web)"

# The depends_on init container calls the Kubernetes API, which must stay
# reachable through the NetworkPolicy egress rules.
init_exit="$(kubectl_ns get pod "$web_pod" \
    -o jsonpath='{.status.initContainerStatuses[?(@.name=="wait-for-db")].state.terminated.exitCode}')"
assert_equals "0" "$init_exit" "the depends-on init container should reach the Kubernetes API"

# The application must be able to reach its dependency.
db_service="$(service_name db)"
if ! kubectl_ns exec "$web_pod" -c web -- nc -z -w 2 "$db_service" 6379 >/dev/null 2>&1; then
    die "the web pod cannot reach db:6379 through the NetworkPolicy"
fi

# A pod without the application labels must be blocked by the db ingress rule.
kubectl_ns run policy-probe \
    --image=busybox:1.38.0 \
    --restart=Never \
    --command -- sleep 3600 >/dev/null
kubectl_ns wait --for=condition=Ready pod/policy-probe --timeout=60s >/dev/null

if kubectl_ns exec policy-probe -- nc -z -w 2 "$db_service" 6379 >/dev/null 2>&1; then
    die "an unlabelled pod should not reach db:6379"
fi
