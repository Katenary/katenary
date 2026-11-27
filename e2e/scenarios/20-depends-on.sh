#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/depends-on/compose.yaml"
chart="$CACHE_DIR/charts/$SCENARIO"

katenary_convert "$fixture" "$chart"
helm_install "$chart"

wait_pod_ready db
wait_pod_ready web

web_pod="$(pod_name web)"
db_pod="$(pod_name db)"

# The compose healthcheck must be converted to a readiness probe, because the
# Kubernetes API depends-on method waits for readyReplicas.
db_deployment="$(kubectl_ns get deployments \
    --selector=app.kubernetes.io/component=db \
    -o jsonpath='{.items[0].metadata.name}')"
probe_command="$(kubectl_ns get deployment "$db_deployment" \
    -o jsonpath='{.spec.template.spec.containers[0].readinessProbe.exec.command}')"
assert_contains "$probe_command" "/bin/sh" "the compose healthcheck should produce a shell readiness probe"

liveness="$(kubectl_ns get deployment "$db_deployment" \
    -o jsonpath='{.spec.template.spec.containers[0].livenessProbe}')"
if [[ -n "$liveness" ]]; then
    die "the compose healthcheck must not produce a liveness probe"
fi

init_exit="$(kubectl_ns get pod "$web_pod" \
    -o jsonpath='{.status.initContainerStatuses[?(@.name=="wait-for-db")].state.terminated.exitCode}')"
assert_equals "0" "$init_exit" "the wait-for-db init container should succeed"

# The Kubernetes API method needs a ServiceAccount, a Role and a RoleBinding.
for kind in serviceaccount role rolebinding; do
    count="$(kubectl_ns get "$kind" -l app.kubernetes.io/component=web -o name | grep -c . || true)"
    if [[ "$count" -lt 1 ]]; then
        die "missing $kind for the web service"
    fi
done

db_ready="$(kubectl_ns get pod "$db_pod" \
    -o jsonpath='{.status.conditions[?(@.type=="Ready")].lastTransitionTime}')"
init_done="$(kubectl_ns get pod "$web_pod" \
    -o jsonpath='{.status.initContainerStatuses[?(@.name=="wait-for-db")].state.terminated.finishedAt}')"

[[ -n "$db_ready" && -n "$init_done" ]] || die "cannot read the readiness timestamps"

# RFC3339 timestamps in the same timezone can be compared as strings.
if [[ "$init_done" < "$db_ready" ]]; then
    die "web started before db was ready (init done: $init_done, db ready: $db_ready)"
fi

log "db ready at $db_ready, web init container finished at $init_done"
