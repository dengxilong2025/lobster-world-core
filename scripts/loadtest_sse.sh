#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
WORLD_ID="${WORLD_ID:-w_load}"
CONNECTIONS="${CONNECTIONS:-10}"
DURATION_SEC="${DURATION_SEC:-10}"

echo "== loadtest_sse =="
echo "BASE_URL=$BASE_URL"
echo "WORLD_ID=$WORLD_ID"
echo "CONNECTIONS=$CONNECTIONS"
echo "DURATION_SEC=$DURATION_SEC"

tmp="$(mktemp)"
cleanup() { rm -f "$tmp"; }
trap cleanup EXIT

# Each worker prints: "<exit_code> <data_lines>"
seq "$CONNECTIONS" | xargs -P "$CONNECTIONS" -I{} bash -c '
  set -euo pipefail
  url="'"$BASE_URL"'/api/v0/events/stream?world_id='"$WORLD_ID"'"
  # Count SSE data lines as a proxy for event throughput.
  # timeout exit code: 124 means timed out (expected).
  out=$(timeout "'"$DURATION_SEC"'s" bash -c "curl -sN \"$url\" | grep -c \"^data: \"" || true)
  rc=$?
  echo "$rc $out"
' >"$tmp"

echo "RESULTS (per connection):"
sed 's/^/ /' "$tmp"

echo "SUMMARY:"
awk '
  { rc[$1]++; sum+=$2; n++; }
  END{
    printf("  CONNECTIONS=%d\n", n);
    printf("  TOTAL_DATA_LINES=%d\n", sum);
    printf("  AVG_DATA_LINES_PER_CONN=%.2f\n", (n>0? sum/n : 0));
    printf("  EXIT_CODES:\n");
    for (k in rc) printf("    %s %d\n", k, rc[k]);
  }
' "$tmp"

echo "Tip: exit code 124 is expected (timeout). Non-124 non-0 often indicates early disconnect."

