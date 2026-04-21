#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

BASE_URL="http://localhost:8080"
WORLD_ID="w1"
N=3
EXPORT_LIMIT=200

# A small default goal set to avoid brittle runs.
GOALS=(
  "去狩猎获取食物"
  "组织集市交换物资"
  "修缮城墙并训练守卫"
)

usage() {
  cat <<'EOF'
v0.2-M2 批测脚本（v0）

用法：
  bash scripts/agent_test_v0_2_m2.sh [--base-url URL] [--world-id ID] [--n N] [--export-limit N] [--goal TEXT]...

示例：
  bash scripts/agent_test_v0_2_m2.sh --base-url http://localhost:8080 --world-id w1 --n 5
  bash scripts/agent_test_v0_2_m2.sh --goal "去狩猎获取食物" --goal "修建水渠" --n 2

产物：
  out/agent_runs/<ts>/
    - intent_<i>.json              # POST /api/v0/intents 响应体（无论成功失败）
    - intent_<i>.status            # POST 状态码
    - home_<i>.json / home_<i>.status
    - export_<i>.ndjson / export_<i>.status
EOF
}

log() { echo "[agent_test] $*"; }

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

# Parse args
USER_GOALS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url)
      BASE_URL="${2:-}"; shift 2;;
    --world-id)
      WORLD_ID="${2:-}"; shift 2;;
    --n)
      N="${2:-}"; shift 2;;
    --export-limit)
      EXPORT_LIMIT="${2:-}"; shift 2;;
    --goal)
      USER_GOALS+=("${2:-}"); shift 2;;
    *)
      echo "未知参数：$1" >&2
      usage
      exit 2;;
  esac
done

if [[ ${#USER_GOALS[@]} -gt 0 ]]; then
  GOALS=("${USER_GOALS[@]}")
fi

if [[ -z "${BASE_URL}" || -z "${WORLD_ID}" ]]; then
  echo "base-url/world-id 不能为空" >&2
  exit 2
fi

TS="$(date +"%Y%m%d-%H%M%S")"
RUN_DIR="${ROOT}/out/agent_runs/${TS}"
mkdir -p "${RUN_DIR}"

log "base_url=${BASE_URL}"
log "world_id=${WORLD_ID}"
log "n=${N}"
log "export_limit=${EXPORT_LIMIT}"
log "run_dir=${RUN_DIR}"

urlencode() {
  # Prefer python3 (widely available), fall back to jq, and lastly to a conservative replacement.
  local s="$1"
  if command -v python3 >/dev/null 2>&1; then
    python3 - <<PY
import urllib.parse
print(urllib.parse.quote("""$s"""))
PY
    return 0
  fi
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$s" | jq -sRr @uri
    return 0
  fi
  # Minimal fallback: space -> %20 (good enough for our default world_id values).
  printf '%s' "$s" | sed 's/ /%20/g'
}

post_intent() {
  local goal="$1"
  local out_json="$2"
  local out_status="$3"

  local payload
  payload="$(printf '{"world_id":"%s","goal":"%s"}' "${WORLD_ID}" "$(echo "${goal}" | sed 's/"/\\"/g')")"
  local code
  code="$(curl -sS -o "${out_json}" -w "%{http_code}" \
    -X POST "${BASE_URL}/api/v0/intents" \
    -H "Content-Type: application/json" \
    --data "${payload}" || true)"
  echo "${code}" > "${out_status}"
  echo "${code}"
}

get_json() {
  local url="$1"
  local out_body="$2"
  local out_status="$3"
  local code
  code="$(curl -sS -o "${out_body}" -w "%{http_code}" "${url}" || true)"
  echo "${code}" > "${out_status}"
  echo "${code}"
}

get_ndjson() {
  local url="$1"
  local out_body="$2"
  local out_status="$3"
  local code
  code="$(curl -sS -o "${out_body}" -w "%{http_code}" "${url}" || true)"
  echo "${code}" > "${out_status}"
  echo "${code}"
}

pick_goal() {
  local i="$1"
  local idx=$(( (i - 1) % ${#GOALS[@]} ))
  echo "${GOALS[$idx]}"
}

for ((i=1; i<=N; i++)); do
  goal="$(pick_goal "${i}")"
  log "run #${i}/${N} goal=${goal}"

  post_code="$(post_intent "${goal}" "${RUN_DIR}/intent_${i}.json" "${RUN_DIR}/intent_${i}.status")"
  log "POST /intents => ${post_code}"

  home_url="${BASE_URL}/api/v0/spectator/home?world_id=$(urlencode "${WORLD_ID}")"
  home_code="$(get_json "${home_url}" "${RUN_DIR}/home_${i}.json" "${RUN_DIR}/home_${i}.status")"
  log "GET /spectator/home => ${home_code}"

  export_url="${BASE_URL}/api/v0/replay/export?world_id=$(urlencode "${WORLD_ID}")&limit=${EXPORT_LIMIT}"
  export_code="$(get_ndjson "${export_url}" "${RUN_DIR}/export_${i}.ndjson" "${RUN_DIR}/export_${i}.status")"
  log "GET /replay/export => ${export_code}"

  # Small delay to allow tick to advance in local fast config.
  sleep 0.2
done

log "DONE"
