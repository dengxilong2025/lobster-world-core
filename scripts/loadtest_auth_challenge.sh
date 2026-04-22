#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
CONCURRENCY="${CONCURRENCY:-10}"
REQUESTS="${REQUESTS:-200}"

tmp="$(mktemp)"
cleanup() { rm -f "$tmp"; }
trap cleanup EXIT

payload='{"wallet":"w_test"}'

start_ns="$(date +%s%N)"
seq "$REQUESTS" | xargs -P "$CONCURRENCY" -I{} bash -c '
  curl -sS -o /dev/null \
    -w "%{http_code} %{time_total}\n" \
    -H "Content-Type: application/json" \
    -X POST "'"$BASE_URL"'/api/v0/auth/challenge" \
    --data '"'"$payload"'"'
' >"$tmp"
end_ns="$(date +%s%N)"

dur_s="$(awk -v s="$start_ns" -v e="$end_ns" 'BEGIN{printf "%.3f", (e-s)/1000000000.0}')"

echo "== loadtest_auth_challenge =="
echo "BASE_URL=$BASE_URL"
echo "CONCURRENCY=$CONCURRENCY"
echo "REQUESTS=$REQUESTS"
echo "DURATION_SEC=$dur_s"

awk -v dur="$dur_s" '
  { c[$1]++; t+=$2; n++; }
  END{
    printf("TOTAL=%d\n", n);
    printf("QPS≈%.2f\n", (dur>0? n/dur : 0));
    printf("AVG_TIME_SEC=%.4f\n", (n>0? t/n : 0));
    printf("STATUS_COUNTS:\n");
    for (k in c) printf("  %s %d\n", k, c[k]);
  }
' "$tmp" | sort -k1,1

echo "Tip: if you see 429, this is expected due to auth rate limiting."

