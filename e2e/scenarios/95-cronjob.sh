#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib.sh"

fixture="$E2E_ROOT/fixtures/cronjob/compose.yaml"
chart="$CACHE_DIR/charts/$SCENARIO"

katenary_convert "$fixture" "$chart"
helm_install "$chart"

cronjob="$(kubectl_ns get cronjobs -l app.kubernetes.io/component=cron -o jsonpath='{.items[0].metadata.name}')"
[[ -n "$cronjob" ]] || die "the CronJob was not created"

schedule="$(kubectl_ns get cronjob "$cronjob" -o jsonpath='{.spec.schedule}')"
assert_equals "*/5 * * * *" "$schedule" "unexpected CronJob schedule"

for kind in serviceaccount role rolebinding; do
    count="$(kubectl_ns get "$kind" -l app.kubernetes.io/component=cron -o name | grep -c . || true)"
    if [[ "$count" -lt 1 ]]; then
        die "missing $kind for the cron service"
    fi
done

# Run the CronJob template immediately instead of waiting for the schedule.
job="manual-${SCENARIO}"
kubectl_ns create job --from="cronjob/$cronjob" "$job" >/dev/null
kubectl_ns wait --for=condition=Complete "job/$job" --timeout=120s >/dev/null

logs="$(kubectl_ns logs "job/$job" --all-containers)"
assert_contains "$logs" "cronjob-ok" "the job command did not run"
