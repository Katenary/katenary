#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/persistence/compose.yaml"
chart="$CACHE_DIR/charts/$SCENARIO"

katenary_convert "$fixture" "$chart"
helm_install "$chart"

wait_pod_ready web

pvc="$(kubectl_ns get pvc -l app.kubernetes.io/component=web -o jsonpath='{.items[0].metadata.name}')"
[[ -n "$pvc" ]] || die "the PVC was not created"

phase="$(kubectl_ns get pvc "$pvc" -o jsonpath='{.status.phase}')"
assert_equals "Bound" "$phase" "the PVC should be bound"

old_pod="$(pod_name web)"
kubectl_ns exec "$old_pod" -c web -- sh -c 'echo persisted > /data/file'

kubectl_ns delete pod "$old_pod" --wait=true >/dev/null
wait_pod_ready web

new_pod="$(pod_name web)"
if [[ "$new_pod" == "$old_pod" ]]; then
    die "the pod was not recreated"
fi

content="$(kubectl_ns exec "$new_pod" -c web -- cat /data/file | tr -d '\r\n')"
assert_equals "persisted" "$content" "the data did not survive the pod recreation"
