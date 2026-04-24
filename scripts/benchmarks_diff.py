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

def _get_metrics(d: Dict[str, Any]) -> Dict[str, Any]:
    return ((d.get("snapshots") or {}).get("debug_metrics") or {})


def _sum_tick_overrun_total(d: Dict[str, Any]) -> Optional[int]:
    m = _get_metrics(d)
    wts = m.get("world_tick_stats") or {}
    if not isinstance(wts, dict) or not wts:
        return None
    s = 0
    found = False
    for _, v in wts.items():
        if not isinstance(v, dict):
            continue
        x = v.get("tick_overrun_total")
        if isinstance(x, (int, float)):
            s += int(x)
            found = True
    return s if found else None


def _bench_world_id(d: Dict[str, Any]) -> Optional[str]:
    intents = _get_test(d, "intents")
    wid = intents.get("world_id")
    if isinstance(wid, str) and wid:
        return wid
    sha = (d.get("meta") or {}).get("sha")
    if isinstance(sha, str) and sha:
        return f"w_bench_{sha}"
    return None


def _bench_pending_queue_len(d: Dict[str, Any]) -> Optional[int]:
    wid = _bench_world_id(d)
    if not wid:
        return None
    m = _get_metrics(d)
    wqs = m.get("world_queue_stats") or {}
    if not isinstance(wqs, dict):
        return None
    st = wqs.get(wid)
    if not isinstance(st, dict):
        return None
    v = st.get("pending_queue_len")
    if isinstance(v, (int, float)):
        return int(v)
    return None


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

    # agent_batch (v2.3)
    cur_ab = _get_test(current, "agent_batch")
    base_ab = _get_test(baseline, "agent_batch")
    if cur_ab or base_ab:
        lines.append("## agent_batch")
        lines.append("")
        rows2: List[List[str]] = []

        r, reg = _metric_row("duration_sec", cur_ab.get("duration_sec"), base_ab.get("duration_sec"), "increase", threshold_pct)
        rows2.append(r)
        any_reg = any_reg or reg

        # fail_total: absolute increase is a regression (more intuitive than pct threshold)
        b_fail = base_ab.get("fail_total")
        c_fail = cur_ab.get("fail_total")
        r, _ = _metric_row("fail_total", c_fail, b_fail, None, threshold_pct)
        r[3] = "—"
        # verdict: regression if cur > base (or base missing but cur > 0)
        if isinstance(c_fail, (int, float)) and isinstance(b_fail, (int, float)):
            r[4] = "REGRESSION" if int(c_fail) > int(b_fail) else "OK"
        elif isinstance(c_fail, (int, float)) and int(c_fail) > 0:
            r[4] = "REGRESSION"
        else:
            r[4] = "—"
        rows2.append(r)
        if r[4] == "REGRESSION":
            any_reg = True

        # show export volumes (no verdict)
        for k in ["export_lines_total", "export_bytes_total"]:
            r, _ = _metric_row(k, cur_ab.get(k), base_ab.get(k), None, threshold_pct)
            r[3] = "—"
            r[4] = "—"
            rows2.append(r)

        lines.append(_table(rows2))
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

    # Extra explanatory summaries (v2.2)
    lines.append("## world_tick_stats summary")
    lines.append("")
    base_overrun = _sum_tick_overrun_total(baseline)
    cur_overrun = _sum_tick_overrun_total(current)
    lines.append(
        f"tick_overrun_total_sum: {base_overrun if base_overrun is not None else 'n/a'} → {cur_overrun if cur_overrun is not None else 'n/a'}"
    )
    lines.append("")

    lines.append("## queue depth summary")
    lines.append("")
    base_q = _bench_pending_queue_len(baseline)
    cur_q = _bench_pending_queue_len(current)
    lines.append(
        f"bench_world_pending_queue_len: {base_q if base_q is not None else 'n/a'} → {cur_q if cur_q is not None else 'n/a'}"
    )
    lines.append("")

    if any_reg:
        lines.append("## Verdict")
        lines.append("")
        lines.append("REGRESSION detected.")
        lines.append("")

    return "\n".join(lines)
