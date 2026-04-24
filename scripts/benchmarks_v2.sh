#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
THRESHOLD_PCT="${THRESHOLD_PCT:-10}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

date_ymd="$(date +%F)"
sha="$(git rev-parse --short HEAD)"

out_dir="docs/ops/benchmarks"
mkdir -p "$out_dir"

# Non-overwrite naming: if basename already exists, append _HHMMSS.
base_path="${out_dir}/${date_ymd}_${sha}"
if [[ -f "${base_path}.json" ]]; then
  base_path="${base_path}_$(date +%H%M%S)"
fi
json_out="${base_path}.json"
md_out="${base_path}.md"
diff_out="${base_path}.diff.md"

tmpdir="$(mktemp -d)"
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT

echo "[bench v2] BASE_URL=${BASE_URL} sha=${sha} date=${date_ymd}"

# baseline: pick the latest existing json (before writing current), excluding current.
baseline_json=""
if ls "${out_dir}"/*.json >/dev/null 2>&1; then
  baseline_json="$(python3 ./scripts/benchmarks_baseline.py --out-dir "${out_dir}" --current "${json_out}" || true)"
fi

# Run loadtests (capture raw outputs).
auth_out="${tmpdir}/auth.txt"
intents_out="${tmpdir}/intents.txt"
export_out="${tmpdir}/export.txt"
agent_out="${tmpdir}/agent_batch.json"

BASE_URL="$BASE_URL" CONCURRENCY=10 REQUESTS=200 ./scripts/loadtest_auth_challenge.sh >"$auth_out"
BASE_URL="$BASE_URL" WORLD_ID="w_bench_${sha}" CONCURRENCY=50 REQUESTS=500 GOAL="启动世界" ./scripts/loadtest_intents.sh >"$intents_out"
BASE_URL="$BASE_URL" WORLD_ID="w_bench_${sha}" CONCURRENCY=5 REQUESTS=50 LIMIT=5000 ./scripts/loadtest_replay_export.sh >"$export_out"

# Optional: agent batch-run (v0.2-M2). Default enabled for v2.3: set RUN_AGENT=0 to skip.
RUN_AGENT="${RUN_AGENT:-1}"
AGENT_N="${AGENT_N:-3}"
AGENT_EXPORT_LIMIT="${AGENT_EXPORT_LIMIT:-500}"
if [[ "${RUN_AGENT}" == "1" ]]; then
  echo "[bench v2] agent_batch: n=${AGENT_N} export_limit=${AGENT_EXPORT_LIMIT}"
  set +e
  BASE_URL="$BASE_URL" AGENT_N="$AGENT_N" AGENT_EXPORT_LIMIT="$AGENT_EXPORT_LIMIT" \
    python3 ./scripts/benchmarks_agent_v0_2_m2.py --base-url "$BASE_URL" --n "$AGENT_N" --export-limit "$AGENT_EXPORT_LIMIT" --out "$agent_out"
  rc=$?
  set -e
  if [[ "$rc" != "0" ]]; then
    echo "[bench v2] agent_batch failed (rc=$rc), continuing (see ${agent_out})"
  fi
else
  echo "[bench v2] agent_batch skipped (RUN_AGENT=0)"
fi

# Fetch snapshots.
cfg_json="${tmpdir}/debug_config.json"
metrics_json="${tmpdir}/debug_metrics.json"
curl -s "${BASE_URL}/api/v0/debug/config" >"$cfg_json"
curl -s "${BASE_URL}/api/v0/debug/metrics" >"$metrics_json"

go_version="$(go version 2>/dev/null || true)"
uname_a="$(uname -a 2>/dev/null || true)"

# Build benchmark json.
python3 - "$auth_out" "$intents_out" "$export_out" "$agent_out" "$cfg_json" "$metrics_json" "$json_out" <<'PY'
import json, sys
from pathlib import Path

from scripts.benchmarks_parse import parse_loadtest_output

auth_out, intents_out, export_out, agent_out, cfg_json, metrics_json, json_out = sys.argv[1:]

def read(p: str) -> str:
    return Path(p).read_text(encoding="utf-8", errors="replace")

cfg = json.loads(read(cfg_json) or "{}")
metrics_raw = json.loads(read(metrics_json) or "{}")
metrics = (metrics_raw.get("metrics") or {})

out = {
    "meta": {
        "date": __import__("datetime").date.today().isoformat(),
        "sha": __import__("subprocess").check_output(["git","rev-parse","--short","HEAD"]).decode().strip(),
        "base_url": __import__("os").environ.get("BASE_URL",""),
        "go_version": __import__("subprocess").getoutput("go version"),
        "uname": __import__("subprocess").getoutput("uname -a"),
        "threshold_regression_pct": int(__import__("os").environ.get("THRESHOLD_PCT","10")),
    },
    "tests": {
        "auth_challenge": parse_loadtest_output(read(auth_out)),
        "intents": parse_loadtest_output(read(intents_out)),
        "replay_export": parse_loadtest_output(read(export_out)),
    },
    "snapshots": {
        "debug_config": cfg.get("config", cfg),
        "debug_metrics": {
            "busy_by_reason": metrics.get("busy_by_reason", {}),
            "world_queue_stats": metrics.get("world_queue_stats", {}),
            "world_tick_stats": metrics.get("world_tick_stats", {}),
            "responses_by_status": metrics.get("responses_by_status", {}),
        },
    },
}

try:
    ap = Path(agent_out)
    if ap.exists() and ap.stat().st_size > 0:
        out["tests"]["agent_batch"] = json.loads(read(agent_out))
except Exception:
    pass

Path(json_out).write_text(json.dumps(out, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
PY

# Build markdown report from json (simple human-readable).
python3 - "$json_out" "$md_out" <<'PY'
import json, sys
from pathlib import Path

src, dst = sys.argv[1:]
d = json.loads(Path(src).read_text(encoding="utf-8"))
meta = d.get("meta", {})
tests = d.get("tests", {})
snap = (d.get("snapshots") or {}).get("debug_metrics") or {}

lines = []
lines.append(f"# Benchmarks v2（{meta.get('date','')} {meta.get('sha','')}）")
lines.append("")
lines.append(f"- BASE_URL: `{meta.get('base_url','')}`")
lines.append(f"- threshold_regression_pct: {meta.get('threshold_regression_pct',10)}%")
lines.append("")

def add_test(name: str):
    t = tests.get(name) or {}
    lines.append(f"## {name}")
    lines.append("")
    for k in ["duration_sec","total","qps","avg_time_sec","avg_bytes","busy_503"]:
        if k in t:
            lines.append(f"- {k}: {t[k]}")
    sc = t.get("status_counts") or {}
    if sc:
        lines.append(f"- status_counts: {sc}")
    lines.append("")

for n in ["auth_challenge","intents","replay_export"]:
    add_test(n)

if tests.get("agent_batch") is not None:
    lines.append("## agent_batch")
    lines.append("")
    ab = tests.get("agent_batch") or {}
    # Show only a compact subset.
    for k in ["duration_sec","fail_total","export_lines_total","export_bytes_total","world_id","n"]:
        if k in ab:
            lines.append(f"- {k}: {ab[k]}")
    lines.append("")

lines.append("## busy_by_reason")
lines.append("")
lines.append("```")
lines.append(str(snap.get("busy_by_reason", {})))
lines.append("```")
lines.append("")

lines.append("## world_tick_stats（摘要）")
lines.append("")
lines.append("```")
lines.append(str(snap.get("world_tick_stats", {})))
lines.append("```")
lines.append("")

Path(dst).write_text("\n".join(lines) + "\n", encoding="utf-8")
PY

# Build diff report.
if [[ -n "$baseline_json" ]] && [[ -f "$baseline_json" ]]; then
  echo "[bench v2] baseline: ${baseline_json}"
  python3 - "$json_out" "$baseline_json" "$diff_out" <<'PY'
import json, sys
from pathlib import Path

from scripts.benchmarks_diff import diff_summary

cur_p, base_p, out_p = sys.argv[1:]
cur = json.loads(Path(cur_p).read_text(encoding="utf-8"))
base = json.loads(Path(base_p).read_text(encoding="utf-8"))
threshold = int((cur.get("meta") or {}).get("threshold_regression_pct", 10))
md = diff_summary(cur, base, threshold_pct=threshold)
header = []
header.append(f"- current: `{Path(cur_p).name}`")
header.append(f"- baseline: `{Path(base_p).name}`")
header.append("")
md = "\n".join(header) + md
Path(out_p).write_text(md + "\n", encoding="utf-8")
PY
else
  cat >"$diff_out" <<EOF
# Benchmarks Diff Summary

no baseline found
EOF
fi

echo "[bench v2] wrote:"
echo "  - ${json_out}"
echo "  - ${md_out}"
echo "  - ${diff_out}"
