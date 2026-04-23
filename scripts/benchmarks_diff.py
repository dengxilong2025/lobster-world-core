from __future__ import annotations

from typing import Any, Dict, List, Optional, Tuple


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


def _metric_row(
    metric: str,
    cur: float | int | None,
    base: float | int | None,
    bad_when: Optional[str],
    threshold: int,
) -> Tuple[List[str], bool]:
    """
    Returns: [metric, baseline, current, delta, verdict], regression?
    bad_when: 'decrease' for QPS, 'increase' for AVG_TIME, or None for no verdict.
    """
    if cur is None or base is None:
        return [metric, "n/a", "n/a", "n/a", "—"], False
    cur_f = float(cur)
    base_f = float(base)
    pct = _pct_change(cur_f, base_f)
    flag = False
    if pct is not None and bad_when is not None:
        if bad_when == "decrease" and pct <= -threshold:
            flag = True
        if bad_when == "increase" and pct >= threshold:
            flag = True
    verdict = "REGRESSION" if flag else ("OK" if bad_when is not None else "—")
    return [metric, str(base), str(cur), _fmt_pct(pct), verdict], flag


def _table(rows: List[List[str]]) -> str:
    # Markdown table: assume 5 columns.
    out: List[str] = []
    out.append("| metric | baseline | current | delta | verdict |")
    out.append("|---|---:|---:|---:|---|")
    for r in rows:
        out.append(f"| {r[0]} | {r[1]} | {r[2]} | {r[3]} | {r[4]} |")
    return "\n".join(out)


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
        rows: List[List[str]] = []

        r, reg = _metric_row("qps", cur.get("qps"), base.get("qps"), "decrease", threshold_pct)
        rows.append(r)
        any_reg = any_reg or reg

        r, reg = _metric_row("avg_time_sec", cur.get("avg_time_sec"), base.get("avg_time_sec"), "increase", threshold_pct)
        rows.append(r)
        any_reg = any_reg or reg

        # Always show busy_503 if present (no verdict)
        if "busy_503" in cur or "busy_503" in base:
            r, _ = _metric_row("busy_503", cur.get("busy_503"), base.get("busy_503"), None, threshold_pct)
            r[3] = "—"
            r[4] = "—"
            rows.append(r)

        # status summary (200/429/503)
        cur_sc = cur.get("status_counts") or {}
        base_sc = base.get("status_counts") or {}
        if cur_sc or base_sc:
            def fmt(sc: Dict[str, Any]) -> str:
                return f"{int(sc.get('200',0))}/{int(sc.get('429',0))}/{int(sc.get('503',0))}"
            rows.append(["status 200/429/503", fmt(base_sc), fmt(cur_sc), "—", "—"])

        lines.append(_table(rows))
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
