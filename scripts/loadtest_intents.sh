#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
WORLD_ID="${WORLD_ID:-w_load}"
CONCURRENCY="${CONCURRENCY:-10}"
REQUESTS="${REQUESTS:-200}"
GOAL="${GOAL:-启动世界}"

tmp="$(mktemp)"
cleanup() { rm -f "$tmp"; }
trap cleanup EXIT

start_ns="$(date +%s%N)"
seq "$REQUESTS" | xargs -P "$CONCURRENCY" -I{} bash -c '
  goal="'"$GOAL"' #{}"
  body=$(printf "{\"world_id\":\"%s\",\"goal\":\"%s\"}" "'"$WORLD_ID"'" "$goal")
  curl -sS -o /dev/null \
    -w "%{http_code} %{time_total}\n" \
    -H "Content-Type: application/json" \
    -X POST "'"$BASE_URL"'/api/v0/intents" \
    --data "$body"
' >"$tmp"
end_ns="$(date +%s%N)"

dur_s="$(awk -v s="$start_ns" -v e="$end_ns" 'BEGIN{printf "%.3f", (e-s)/1000000000.0}')"

echo "== loadtest_intents =="
echo "BASE_URL=$BASE_URL"
echo "WORLD_ID=$WORLD_ID"
echo "CONCURRENCY=$CONCURRENCY"
echo "REQUESTS=$REQUESTS"
echo "GOAL=$GOAL"
echo "DURATION_SEC=$dur_s"

summary="$(awk -v dur="$dur_s" '
  { c[$1]++; t+=$2; n++; if ($1=="503") busy++; }
  END{
    printf("TOTAL=%d\n", n);
    printf("QPS≈%.2f\n", (dur>0? n/dur : 0));
    printf("AVG_TIME_SEC=%.4f\n", (n>0? t/n : 0));
    printf("STATUS_COUNTS:\n");
    for (k in c) printf("  %s %d\n", k, c[k]);
    printf("BUSY_503=%d\n", busy+0);
  }
' "$tmp")"

echo "$summary" | sed 's/^/ /'

if echo "$summary" | grep -q '^BUSY_503=[1-9]'; then
  echo "Tip: 503 BUSY indicates sim queue backpressure. Check /api/v0/debug/metrics busy_total and consider lowering CONCURRENCY or increasing MaxIntentQueue."
fi

