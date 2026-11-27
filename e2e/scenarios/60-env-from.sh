#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/env-from/compose.yaml"
chart="$CACHE_DIR/charts/$SCENARIO"

katenary_convert "$fixture" "$chart"
helm_install "$chart"

wait_pod_ready web
wait_pod_ready db

db_pod="$(pod_name db)"

foo="$(kubectl_ns exec "$db_pod" -c db -- printenv FOO | tr -d '\r\n')"
assert_equals "bar" "$foo" "env-from did not inject the web environment"

password="$(kubectl_ns exec "$db_pod" -c db -- printenv DB_PASSWORD | tr -d '\r\n')"
assert_equals "topsecret" "$password" "values-from did not bind the web secret"

for kind in secret configmap; do
    count="$(kubectl_ns get "$kind" -l app.kubernetes.io/component=web -o name | grep -c . || true)"
    if [[ "$count" -lt 1 ]]; then
        die "missing $kind for web"
    fi
done

db_deployment="$(kubectl_ns get deployments \
    --selector=app.kubernetes.io/component=db \
    -o jsonpath='{.items[0].metadata.name}')"

env_from="$(kubectl_ns get deployment "$db_deployment" \
    -o jsonpath='{.spec.template.spec.containers[0].envFrom[0].configMapRef.name}')"
[[ -n "$env_from" ]] || die "db does not read the web ConfigMap with envFrom"

secret_ref="$(kubectl_ns get deployment "$db_deployment" \
    -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="DB_PASSWORD")].valueFrom.secretKeyRef.name}')"
[[ -n "$secret_ref" ]] || die "DB_PASSWORD is not bound from the web Secret"
