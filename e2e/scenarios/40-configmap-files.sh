#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/configmap-files/compose.yaml"
chart="$CACHE_DIR/charts/$SCENARIO"

katenary_convert "$fixture" "$chart"
helm_install "$chart"

wait_pod_ready web

configmaps="$(kubectl_ns get configmap -l app.kubernetes.io/component=web -o name | grep -c . || true)"
assert_equals "3" "$configmaps" "expected one configmap for the nested file, one for the root file and one for the directory"

keys="$(kubectl_ns get configmap -l app.kubernetes.io/component=web \
    -o go-template='{{range .items}}{{range $k, $v := .data}}{{$k}}{{"\n"}}{{end}}{{end}}')"
assert_contains "$keys" "default.conf" "the nested file is missing from the configmaps"
assert_contains "$keys" "message.txt" "the root file is missing from the configmaps"
assert_contains "$keys" "a.txt" "the directory files are missing from the configmaps"
assert_contains "$keys" "b.txt" "the directory files are missing from the configmaps"

pod="$(pod_name web)"

mounted="$(kubectl_ns exec "$pod" -c web -- cat /etc/nginx/conf.d/default.conf)"
assert_contains "$mounted" "from-configmap" "the mounted nginx configuration is wrong"

message="$(kubectl_ns exec "$pod" -c web -- cat /usr/share/nginx/html/message.txt | tr -d '\r\n')"
assert_equals "hello from configmap" "$message" "the mounted root file is wrong"

mounts="$(kubectl_ns get pod "$pod" \
    -o jsonpath='{range .spec.containers[0].volumeMounts[*]}{.mountPath}{":"}{.subPath}{"\n"}{end}')"
assert_contains "$mounts" "/etc/nginx/conf.d/default.conf:default.conf" "unexpected nested file mount"
assert_contains "$mounts" "/usr/share/nginx/html/message.txt:message.txt" "unexpected root file mount"
assert_contains "$mounts" "/usr/share/nginx/html/static:" "unexpected directory mount"

service="$(service_name web)"
start_port_forward "$service" 18081

wait_http "http://127.0.0.1:18081/custom" "from-configmap" >/dev/null
wait_http "http://127.0.0.1:18081/message.txt" "hello from configmap" >/dev/null
wait_http "http://127.0.0.1:18081/static/a.txt" "directory file a" >/dev/null
wait_http "http://127.0.0.1:18081/static/b.txt" "directory file b" >/dev/null
