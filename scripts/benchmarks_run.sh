#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

date_ymd="$(date +%F)"
sha="$(git rev-parse --short HEAD)"
out_dir="docs/ops/benchmarks"
out_file="${out_dir}/${date_ymd}_${sha}.md"

mkdir -p "$out_dir"

tmp="$(mktemp)"
cleanup() { rm -f "$tmp"; }
trap cleanup EXIT

go_version="$(go version 2>/dev/null || true)"
uname_a="$(uname -a 2>/dev/null || true)"

{
  echo "# Benchmarks 归档（${date_ymd} ${sha}）"
  echo
  echo "BASE_URL: \`${BASE_URL}\`"
  echo
  echo "## 环境"
  echo
  echo "- git sha: \`${sha}\`"
  echo "- go: \`${go_version}\`"
  echo "- uname: \`${uname_a}\`"
  echo
  echo "## debug/config"
  echo
  echo '```json'
  curl -s "${BASE_URL}/api/v0/debug/config" | head -c 4000
  echo
  echo '```'
  echo
  echo "## loadtest: auth challenge（小）"
  echo
  echo '```'
  BASE_URL="$BASE_URL" CONCURRENCY=10 REQUESTS=200 ./scripts/loadtest_auth_challenge.sh | head -n 80
  echo '```'
  echo
  echo "## loadtest: intents（中）"
  echo
  echo '```'
  BASE_URL="$BASE_URL" WORLD_ID="w_bench_${sha}" CONCURRENCY=50 REQUESTS=500 GOAL="启动世界" ./scripts/loadtest_intents.sh | head -n 120
  echo '```'
  echo
  echo "## loadtest: replay/export（小）"
  echo
  echo '```'
  BASE_URL="$BASE_URL" WORLD_ID="w_bench_${sha}" CONCURRENCY=5 REQUESTS=50 LIMIT=5000 ./scripts/loadtest_replay_export.sh | head -n 120
  echo '```'
  echo
  echo "## debug/metrics（快照）"
  echo
  echo '```json'
  curl -s "${BASE_URL}/api/v0/debug/metrics" | head -c 8000
  echo
  echo '```'
  echo
} >"$tmp"

mv "$tmp" "$out_file"

echo "Wrote ${out_file}"

