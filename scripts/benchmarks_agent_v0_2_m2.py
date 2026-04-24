#!/usr/bin/env python3
import argparse
import json
import os
import re
import subprocess
from pathlib import Path
from typing import Any, Dict, Optional


def _extract_run_dir(output: str) -> Optional[str]:
    # agent_test prints: [agent_test] run_dir=/path/to/out/agent_runs/<ts>
    m = re.search(r"^\[agent_test\]\s+run_dir=(.+)\s*$", output, flags=re.MULTILINE)
    if not m:
        return None
    return m.group(1).strip()


def _read_json(p: Path) -> Dict[str, Any]:
    return json.loads(p.read_text(encoding="utf-8"))


def _fail_total(summary: Dict[str, Any]) -> int:
    f = summary.get("fail") or {}
    if not isinstance(f, dict):
        return 0
    total = 0
    for k in ["intents", "home", "export"]:
        v = f.get(k, 0)
        if isinstance(v, (int, float)):
            total += int(v)
    return total


def run_agent_batch(base_url: str, n: int, export_limit: int) -> Dict[str, Any]:
    cmd = [
        "bash",
        "scripts/agent_test_v0_2_m2.sh",
        "--base-url",
        base_url,
        "--world-id",
        "auto",
        "--n",
        str(n),
        "--export-limit",
        str(export_limit),
    ]
    proc = subprocess.run(
        cmd,
        cwd=str(Path(__file__).resolve().parent.parent),
        capture_output=True,
        text=True,
    )
    out = (proc.stdout or "") + ("\n" + proc.stderr if proc.stderr else "")
    run_dir = _extract_run_dir(out)
    if proc.returncode != 0 or not run_dir:
        return {
            "ok": False,
            "error": "agent_test failed",
            "returncode": proc.returncode,
            "stdout_tail": out[-2000:],
        }

    summary_path = Path(run_dir) / "summary.json"
    if not summary_path.exists():
        return {
            "ok": False,
            "error": "summary.json not found",
            "run_dir": run_dir,
        }

    summary = _read_json(summary_path)

    # Produce a compact object for benchmarks diff.
    out_obj: Dict[str, Any] = {
        "run_id": summary.get("run_id"),
        "world_id": summary.get("world_id"),
        "n": summary.get("n"),
        "duration_sec": summary.get("duration_sec"),
        "export_lines_total": summary.get("export_lines_total"),
        "export_bytes_total": summary.get("export_bytes_total"),
        "ok_counts": summary.get("ok"),
        "fail_counts": summary.get("fail"),
        "fail_by_http_code": summary.get("fail_by_http_code"),
    }
    out_obj["fail_total"] = _fail_total(summary)
    out_obj["ok"] = True
    return out_obj


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base-url", default=os.environ.get("BASE_URL", "http://localhost:8080"))
    ap.add_argument("--n", type=int, default=int(os.environ.get("AGENT_N", "3")))
    ap.add_argument("--export-limit", type=int, default=int(os.environ.get("AGENT_EXPORT_LIMIT", "500")))
    ap.add_argument("--out", default="")
    args = ap.parse_args()

    obj = run_agent_batch(args.base_url, args.n, args.export_limit)
    s = json.dumps(obj, ensure_ascii=False, indent=2) + "\n"
    if args.out:
        Path(args.out).write_text(s, encoding="utf-8")
    else:
        print(s, end="")
    return 0 if obj.get("ok") else 1


if __name__ == "__main__":
    raise SystemExit(main())

