#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
WORLD_ID="${WORLD_ID:-w_load}"
CONCURRENCY="${CONCURRENCY:-5}"
REQUESTS="${REQUESTS:-50}"
LIMIT="${LIMIT:-5000}"

tmp="$(mktemp)"
cleanup() { rm -f "$tmp"; }
trap cleanup EXIT

start_ns="$(date +%s%N)"
seq "$REQUESTS" | xargs -P "$CONCURRENCY" -I{} bash -c '
  url="'"$BASE_URL"'/api/v0/replay/export?world_id='"$WORLD_ID"'&limit='"$LIMIT"'"
  # Emit "<http_code> <time_total> <size_download>"
  curl -sS -o /dev/null \
    -w "%{http_code} %{time_total} %{size_download}\n" \
    "$url"
' >"$tmp"
end_ns="$(date +%s%N)"

dur_s="$(awk -v s="$start_ns" -v e="$end_ns" 'BEGIN{printf "%.3f", (e-s)/1000000000.0}')"

echo "== loadtest_replay_export =="
echo "BASE_URL=$BASE_URL"
echo "WORLD_ID=$WORLD_ID"
echo "LIMIT=$LIMIT"
echo "CONCURRENCY=$CONCURRENCY"
echo "REQUESTS=$REQUESTS"
echo "DURATION_SEC=$dur_s"

awk -v dur="$dur_s" '
  { c[$1]++; t+=$2; sz+=$3; n++; }
  END{
    printf("TOTAL=%d\n", n);
    printf("QPS≈%.2f\n", (dur>0? n/dur : 0));
    printf("AVG_TIME_SEC=%.4f\n", (n>0? t/n : 0));
    printf("AVG_BYTES=%.1f\n", (n>0? sz/n : 0));
    printf("STATUS_COUNTS:\n");
    for (k in c) printf("  %s %d\n", k, c[k]);
  }
' "$tmp" | sed 's/^/ /'

