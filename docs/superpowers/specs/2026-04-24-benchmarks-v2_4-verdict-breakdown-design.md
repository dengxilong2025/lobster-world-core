# Benchmarks v2.4：Verdict 总结行增强（含 agent_batch）Design

## Goal

让 `*.diff.md` 的 `## Verdict` 不只输出一个 “REGRESSION detected.”，而是明确回答：

1) **哪里回归了？**（auth / intents / replay_export / agent_batch / tick / queue）
2) **回归幅度是多少？**（baseline → current，Δ% 或绝对值）
3) **是什么类型的回归？**（latency ↑ / qps ↓ / fail_total ↑）

这样你一眼就能判断“本次回归主要来自压测还是批测”，降低排障成本。

## Non-goals

- 不引入更复杂的打分系统/权重系统
- 不在 v2.4 里调整阈值策略（沿用既有 threshold_pct 与 fail_total 规则）

---

## 1) 输出格式（diff.md）

现状（v2.3）：
```
## Verdict

REGRESSION detected.
```

目标（v2.4）：
```
## Verdict

REGRESSION detected (3).

- intents.avg_time_sec: 0.0023 → 0.0027 (+17.4%)
- replay_export.avg_time_sec: 0.0021 → 0.0038 (+81.0%)
- agent_batch.duration_sec: 2 → 3 (+50.0%)
```

规则：
- `REGRESSION detected (N).` 中 N 为回归条目数
- 回归条目采用“test.metric”命名空间：
  - `auth_challenge.qps / auth_challenge.avg_time_sec`
  - `intents.qps / intents.avg_time_sec`
  - `replay_export.qps / replay_export.avg_time_sec`
  - `agent_batch.duration_sec / agent_batch.fail_total`
- `busy_by_reason / tick_overrun_total_sum / pending_queue_len` 本版本不进入 Verdict（仅展示在各自章节，避免把“解释性指标”误当失败原因）

---

## 2) 实现策略（benchmarks_diff.py）

在构建表格时，额外收集回归条目列表 `regressions: list[str]`：

- 当 `_metric_row(...).regression == True` 时 push 一条：
  - 例如：`intents.avg_time_sec: 0.0023 → 0.0027 (+17.4%)`
- 对 `agent_batch.fail_total`：当 cur > base 时 push：
  - `agent_batch.fail_total: 0 → 2`

最后输出 Verdict：
- 若 regressions 非空：输出 `REGRESSION detected (len(regressions)).` + 列表
- 否则：输出 `OK.`（保持简洁）

---

## 3) 测试门禁

扩展 `scripts/benchmarks_tests/test_diff.py`：
- 使用 samples 现有回归数据，断言：
  - `REGRESSION detected (`
  - `- intents.avg_time_sec:`
  - `- agent_batch.duration_sec:`（若 sample 中触发）

---

## 4) 交付

- commit + push 到 GitHub main
- 生成覆盖包 zip + 最新项目 zip

