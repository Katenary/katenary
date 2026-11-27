#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/depends-on-legacy/compose.yaml"
chart="$CACHE_DIR/charts/$SCENARIO"

katenary_convert "$fixture" "$chart"
helm_install "$chart"

wait_pod_ready db
wait_pod_ready web

web_pod="$(pod_name web)"
db_pod="$(pod_name db)"

init_exit="$(kubectl_ns get pod "$web_pod" \
    -o jsonpath='{.status.initContainerStatuses[?(@.name=="wait-for-db")].state.terminated.exitCode}')"
assert_equals "0" "$init_exit" "the legacy wait-for-db init container should succeed"

init_image="$(kubectl_ns get pod "$web_pod" \
    -o jsonpath='{.spec.initContainers[?(@.name=="wait-for-db")].image}')"
assert_equals "busybox:1.38.0" "$init_image" "the legacy init container image should be pinned"

db_start="$(kubectl_ns get pod "$db_pod" \
    -o jsonpath='{.status.containerStatuses[0].state.running.startedAt}')"
init_done="$(kubectl_ns get pod "$web_pod" \
    -o jsonpath='{.status.initContainerStatuses[?(@.name=="wait-for-db")].state.terminated.finishedAt}')"

[[ -n "$db_start" && -n "$init_done" ]] || die "cannot read the start timestamps"

elapsed=$(( $(rfc3339_epoch "$init_done") - $(rfc3339_epoch "$db_start") ))
if ((elapsed < 8)); then
    die "the legacy init container did not wait for the delayed redis (${elapsed}s)"
fi

log "legacy init container waited ${elapsed}s for redis"
