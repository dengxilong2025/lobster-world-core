import re
from typing import Any, Dict


def _re_float(pattern: str, text: str) -> float | None:
    m = re.search(pattern, text, flags=re.MULTILINE)
    if not m:
        return None
    try:
        return float(m.group(1))
    except Exception:
        return None


def _re_int(pattern: str, text: str) -> int | None:
    m = re.search(pattern, text, flags=re.MULTILINE)
    if not m:
        return None
    try:
        return int(m.group(1))
    except Exception:
        return None


def parse_loadtest_output(text: str) -> Dict[str, Any]:
    """
    Parse output from scripts/loadtest_*.sh into a structured dict.
    Keeps missing fields absent (no fake defaults).
    """
    out: Dict[str, Any] = {}

    dur = _re_float(r"^\s*DURATION_SEC=([0-9.]+)\s*$", text)
    if dur is not None:
        out["duration_sec"] = dur

    total = _re_int(r"^\s*TOTAL=([0-9]+)\s*$", text)
    if total is not None:
        out["total"] = total

    # Example: "QPS≈113.35"
    qps = _re_float(r"^\s*QPS[≈~]=?([0-9.]+)\s*$", text)
    if qps is not None:
        out["qps"] = qps

    avg = _re_float(r"^\s*AVG_TIME_SEC=([0-9.]+)\s*$", text)
    if avg is not None:
        out["avg_time_sec"] = avg

    avg_bytes = _re_float(r"^\s*AVG_BYTES=([0-9.]+)\s*$", text)
    if avg_bytes is not None:
        out["avg_bytes"] = avg_bytes

    busy = _re_int(r"^\s*BUSY_503=([0-9]+)\s*$", text)
    if busy is not None:
        out["busy_503"] = busy

    # STATUS_COUNTS block
    status_counts: Dict[str, int] = {}
    lines = text.splitlines()
    in_block = False
    for ln in lines:
        if re.match(r"^\s*STATUS_COUNTS:\s*$", ln):
            in_block = True
            continue
        if in_block:
            m = re.match(r"^\s*([0-9]{3})\s+([0-9]+)\s*$", ln)
            if m:
                status_counts[m.group(1)] = int(m.group(2))
                continue
            # end when line doesn't look like counts (blank or other field)
            if ln.strip() == "" or re.match(r"^\s*[A-Z_]+=|^\s*BUSY_503=", ln):
                in_block = False

    if status_counts:
        out["status_counts"] = status_counts

    return out

