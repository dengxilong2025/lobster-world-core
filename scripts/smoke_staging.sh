#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-https://lobster-world-core.onrender.com}"
EXPORT_LIMIT="${EXPORT_LIMIT:-200}"
TIMEOUT_SEC="${TIMEOUT_SEC:-20}"

usage() {
  cat <<EOF
staging smoke（轻量验收脚本）

用法：
  BASE_URL=https://lobster-world-core.onrender.com bash scripts/smoke_staging.sh

可配置环境变量：
  BASE_URL        目标服务地址（默认：${BASE_URL}）
  EXPORT_LIMIT    replay/export limit（默认：${EXPORT_LIMIT}）
  TIMEOUT_SEC     curl 超时（默认：${TIMEOUT_SEC}）

检查项：
  - GET  /healthz                              => 200
  - GET  /                                     => 302 Location: /ui
  - GET  /ui                                   => 200
  - GET  /assets/production/manifest.json      => 200
  - POST /api/v0/intents                       => 200 且响应包含 "ok"
  - GET  /api/v0/spectator/home?world_id=...   => 200
  - GET  /api/v0/replay/export?...&limit=...   => 200
EOF
}

log() { echo "[$(date +%H:%M:%S)] $*"; }
ok() { echo "[OK] $*"; }
fail() { echo "[FAIL] $*" >&2; }

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

curl_code() {
  # prints: http_code
  local method="$1"; shift
  local url="$1"; shift
  curl -sS -o /dev/null \
    --max-time "${TIMEOUT_SEC}" \
    -X "${method}" \
    -w "%{http_code}" \
    "$url" "$@" || true
}

curl_headers() {
  local url="$1"
  curl -sS -I --max-time "${TIMEOUT_SEC}" "$url" || true
}

urlencode() {
  local s="$1"
  python3 - <<PY
import urllib.parse
print(urllib.parse.quote("""$s"""))
PY
}

require_code() {
  local got="$1"
  local want="$2"
  local label="$3"
  if [[ "$got" != "$want" ]]; then
    fail "${label}: expected ${want}, got ${got}"
    return 1
  fi
  ok "${label}: ${got}"
}

log "BASE_URL=${BASE_URL}"

# 1) /healthz
code="$(curl_code GET "${BASE_URL}/healthz")"
require_code "$code" "200" "GET /healthz" || exit 1

# 2) / should 302 to /ui
hdr="$(curl_headers "${BASE_URL}/")"
code="$(echo "$hdr" | tr -d '\r' | awk 'toupper($1) ~ /^HTTP\// {print $2}' | tail -n 1)"
loc="$(echo "$hdr" | tr -d '\r' | awk 'tolower($1)=="location:" {print $2}' | tail -n 1)"
if [[ -z "${code}" ]]; then
  fail "GET /: failed to read headers"
  exit 1
fi
if [[ "$code" != "302" ]]; then
  fail "GET /: expected 302, got ${code}"
  exit 1
fi
if [[ "$loc" != "/ui" ]]; then
  fail "GET /: expected Location=/ui, got ${loc:-<empty>}"
  exit 1
fi
ok "GET /: 302 Location=/ui"

# 3) /ui should be 200
code="$(curl_code GET "${BASE_URL}/ui")"
require_code "$code" "200" "GET /ui" || exit 1

# 4) manifest should be 200
code="$(curl_code GET "${BASE_URL}/assets/production/manifest.json")"
require_code "$code" "200" "GET /assets/production/manifest.json" || exit 1

# 5) minimal write loop: intents -> home -> export
ts="$(date +%Y%m%d-%H%M%S)"
world_id="smoke_${ts}"
goal="staging smoke"

payload="$(python3 - <<PY
import json
print(json.dumps({"world_id":"$world_id","goal":"$goal"}, ensure_ascii=False))
PY
)"

tmp_body="$(mktemp)"
cleanup() { rm -f "$tmp_body"; }
trap cleanup EXIT

icode="$(curl -sS -o "$tmp_body" -w "%{http_code}" --max-time "${TIMEOUT_SEC}" \
  -X POST "${BASE_URL}/api/v0/intents" \
  -H "Content-Type: application/json" \
  --data "${payload}" || true)"

if [[ "$icode" != "200" ]]; then
  fail "POST /api/v0/intents: expected 200, got ${icode}"
  head -n 20 "$tmp_body" >&2 || true
  exit 1
fi
if ! grep -Eq '"ok"[[:space:]]*:[[:space:]]*true' "$tmp_body"; then
  fail "POST /api/v0/intents: response missing \"ok\":true"
  head -n 20 "$tmp_body" >&2 || true
  exit 1
fi
ok "POST /api/v0/intents: 200"

wid_enc="$(urlencode "${world_id}")"
home_url="${BASE_URL}/api/v0/spectator/home?world_id=${wid_enc}"
code="$(curl_code GET "${home_url}")"
require_code "$code" "200" "GET /api/v0/spectator/home" || exit 1

export_url="${BASE_URL}/api/v0/replay/export?world_id=${wid_enc}&limit=${EXPORT_LIMIT}"
code="$(curl_code GET "${export_url}")"
require_code "$code" "200" "GET /api/v0/replay/export" || exit 1

echo "ALL OK"
