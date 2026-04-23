from __future__ import annotations

from typing import Any, Dict, Tuple


def _pct_change(cur: float, base: float) -> float | None:
    if base == 0:
        return None
    return (cur - base) / base * 100.0


def _fmt_pct(p: float | None) -> str:
    if p is None:
        return "n/a"
    sign = "+" if p >= 0 else ""
    return f"{sign}{p:.1f}%"


def _get_test(d: Dict[str, Any], name: str) -> Dict[str, Any]:
    return (d.get("tests") or {}).get(name) or {}


def _metric_row(name: str, cur: float | None, base: float | None, bad_when: str, threshold: int) -> Tuple[str, bool]:
    """
    bad_when: 'decrease' for QPS, 'increase' for AVG_TIME
    """
    if cur is None or base is None:
        return f"- {name}: n/a", False
    pct = _pct_change(cur, base)
    flag = False
    if pct is not None:
        if bad_when == "decrease" and pct <= -threshold:
            flag = True
        if bad_when == "increase" and pct >= threshold:
            flag = True
    label = "REGRESSION" if flag else "OK"
    return f"- {name}: {base} → {cur} ({_fmt_pct(pct)}) [{label}]", flag


def diff_summary(current: Dict[str, Any], baseline: Dict[str, Any], threshold_pct: int = 10) -> str:
    lines: list[str] = []
    lines.append("# Benchmarks Diff Summary")
    lines.append("")
    lines.append(f"- threshold_regression_pct: {threshold_pct}%")
    lines.append("")

    any_reg = False
    for test_name in ["auth_challenge", "intents", "replay_export"]:
        cur = _get_test(current, test_name)
        base = _get_test(baseline, test_name)
        lines.append(f"## {test_name}")
        lines.append("")

        row, reg = _metric_row("qps", cur.get("qps"), base.get("qps"), "decrease", threshold_pct)
        lines.append(row)
        any_reg = any_reg or reg
        row, reg = _metric_row(
            "avg_time_sec", cur.get("avg_time_sec"), base.get("avg_time_sec"), "increase", threshold_pct
        )
        lines.append(row)
        any_reg = any_reg or reg

        # Always show busy_503 if present
        if "busy_503" in cur or "busy_503" in base:
            lines.append(f"- busy_503: {base.get('busy_503','n/a')} → {cur.get('busy_503','n/a')}")
        lines.append("")

    # busy_by_reason
    cur_bbr = ((current.get("snapshots") or {}).get("debug_metrics") or {}).get("busy_by_reason") or {}
    base_bbr = ((baseline.get("snapshots") or {}).get("debug_metrics") or {}).get("busy_by_reason") or {}
    lines.append("## busy_by_reason")
    lines.append("")
    keys = sorted(set(cur_bbr.keys()) | set(base_bbr.keys()))
    for k in keys:
        lines.append(f"- {k}: {base_bbr.get(k, 0)} → {cur_bbr.get(k, 0)}")
    lines.append("")

    if any_reg:
        lines.append("## Verdict")
        lines.append("")
        lines.append("REGRESSION detected.")
        lines.append("")

    return "\n".join(lines)

