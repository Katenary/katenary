#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/if/compose.yaml"
chart="$CACHE_DIR/charts/$SCENARIO"

katenary_convert "$fixture" "$chart"

if ! grep -q "feature.enabled (default: true)" "$chart/values.yaml"; then
    die "the if flag is not documented in values.yaml"
fi

helm_install "$chart"
wait_pod_ready web

deployments="$(kubectl_ns get deployments -l app.kubernetes.io/component=web -o name | grep -c . || true)"
assert_equals "1" "$deployments" "the web deployment should exist by default"

# Disabling the flag must remove every resource of the service.
helm_install "$chart" --set feature.enabled=false
kubectl_ns wait --for=delete deployment -l app.kubernetes.io/component=web --timeout=90s >/dev/null

if [[ -n "$(kubectl_ns get deployment -l app.kubernetes.io/component=web -o name)" ]]; then
    die "the deployment should be removed when feature.enabled is false"
fi
if [[ -n "$(kubectl_ns get service -l app.kubernetes.io/component=web -o name)" ]]; then
    die "the service should be removed when feature.enabled is false"
fi

# Re-enabling it must restore the resources.
helm_install "$chart" --set feature.enabled=true
wait_available web
