# Benchmarks v2.3：纳入 v0.2-M2 批测通道（agent_batch）Design

## Goal

把 v0.2-M2 的“智能体批测通道”纳入 benchmarks v2 的归档与 diff，对外形成一条统一的回归证据链：

- benchmarks 每次运行，除 auth/intents/replay_export 外，还会产出 **agent_batch** 指标摘要（来自 `scripts/agent_test_v0_2_m2.sh` 的 `summary.json`）。
- diff 能同时对比“压测指标 + 批测成功率/耗时/导出规模”，避免出现“压测 OK 但批测在抖”的盲区。

## Non-goals

- 不把 `out/agent_runs/<ts>/export_*.ndjson` 复制进 `docs/ops/benchmarks`（避免仓库膨胀）
- 不做 CI gate（只做本地工具链增强 + 回归门禁测试）

---

## 1) 选型（按用户选择）

采用 **方案 C：独立 agent_bench**：

1) 新增独立脚本产出 agent 批测摘要 JSON（可单独运行）
2) `benchmarks_v2.sh` 在生成主 `benchmarks.json` 时，把 agent 摘要合并进 `tests.agent_batch`

优点：解耦、可单独复用；后续也能独立升级 agent 批测，不影响主 benchmarks 的 loadtest 逻辑。

---

## 2) 新增脚本：agent 批测摘要生成器

新增：`scripts/benchmarks_agent_v0_2_m2.py`（或 `.sh` 包装器，建议 python 便于单测）

行为：
- 调用 `scripts/agent_test_v0_2_m2.sh`（推荐 `--world-id auto`，避免污染）
- 读取生成的 `out/agent_runs/<ts>/summary.json`
- 输出一份“更适合 diff 的精简对象”（stdout JSON）

输出 schema（写入 `benchmarks.json` 的 `tests.agent_batch`）：
```json
{
  "run_id": "20260424-050013",
  "world_id": "agent_20260424-050013",
  "n": 10,
  "duration_sec": 12,
  "export_lines_total": 123,
  "export_bytes_total": 45678,
  "ok": {"intents": 10, "home": 10, "export": 10},
  "fail": {"intents": 0, "home": 0, "export": 0},
  "fail_by_http_code": {"429": 2, "503": 1}
}
```

衍生字段（可选，但推荐加入，便于 diff/判回归）：
- `fail_total = fail.intents + fail.home + fail.export`
- `ok_total = ok.intents + ok.home + ok.export`

---

## 3) benchmarks_v2.sh 合并逻辑

在生成 `benchmarks.json` 时：
- 若 agent 脚本执行成功：写入 `tests.agent_batch`
- 若 agent 脚本失败：写入 `tests.agent_batch = {"error":"...","ok":false}`（不影响其它测试归档）

同时在 `benchmarks.md` 增加 `## agent_batch` 小节，展示核心字段。

---

## 4) diff 规则（benchmarks_diff.py）

在现有 diff 结构里新增：
```
## agent_batch
| metric | baseline | current | delta | verdict |
...
```

建议判定：
- `duration_sec`：上升超过阈值（默认 10%）→ REGRESSION
- `fail_total`：只要上升（cur > base）→ REGRESSION（比百分比更直观）
- 其它字段（export_lines_total/export_bytes_total/ok_total）先展示不判定，避免误伤

---

## 5) 测试门禁

扩展 python 单测（`scripts/benchmarks_tests/test_diff.py`）：
- samples 中新增 `tests.agent_batch`（baseline/current 各一份）
- 断言 diff 输出包含 `## agent_batch` 与关键行（duration_sec/fail_total）

---

## 6) 交付形态

- 提交到 GitHub main
- 输出覆盖包 zip（给你本地覆盖）
- 输出最新项目 zip（给你本地覆盖）

