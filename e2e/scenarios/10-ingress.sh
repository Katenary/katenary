#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/ingress/compose.yaml"
chart="$CACHE_DIR/charts/$SCENARIO"

katenary_convert "$fixture" "$chart"
helm_install "$chart"

web_url="http://web.127.0.0.1.nip.io:${E2E_HTTP_PORT}"
api_url="http://api.127.0.0.1.nip.io:${E2E_HTTP_PORT}"

wait_http "$web_url" "Welcome to nginx" >/dev/null
wait_http "$api_url" "Welcome to nginx" >/dev/null

if [[ -z "$(kubectl_ns get ingress -l app.kubernetes.io/component=web -o name)" ]]; then
    die "the standard Ingress was not created for web"
fi
if [[ -z "$(kubectl_ns get ingressroute -l app.kubernetes.io/component=api -o name)" ]]; then
    die "the IngressRoute was not created for api"
fi

# Disabling the ingress in values.yaml must remove the route, while the
# IngressRoute of the other service stays available.
helm_install "$chart" --set web.ingress.enabled=false
kubectl_ns wait \
    --for=delete ingress \
    --selector=app.kubernetes.io/component=web \
    --timeout=60s >/dev/null

wait_http_status "$web_url" 404
wait_http "$api_url" "Welcome to nginx" >/dev/null
