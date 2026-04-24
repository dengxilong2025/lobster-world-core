# 压测体系 v2.2（diff 补齐解释性指标）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 v2.1 diff 表格基础上，新增两段“解释性”指标摘要：`tick_overrun_total_sum` 与 `bench_world_pending_queue_len`，并加 python 单测门禁与两次冒烟验证。

**Architecture:** 扩展 `scripts/benchmarks_diff.py`：从 current/baseline json 的 `snapshots.debug_metrics` 中提取并输出摘要段；扩展样例 json 与单测确保输出稳定；通过 `benchmarks_v2.sh` 连跑两次验证第二次 diff 真实包含摘要。

**Tech Stack:** python3（标准库 json/unittest）、bash。

---

## 0) Files（锁定）

**Modify:**
- `scripts/benchmarks_diff.py`
- `scripts/benchmarks_tests/test_diff.py`
- `scripts/benchmarks_samples/bench_baseline.json`
- `scripts/benchmarks_samples/bench_current.json`

---

## Task 1: TDD — 样例与单测（RED）

- [ ] **Step 1: 扩展样例 json（baseline/current）**

在 `scripts/benchmarks_samples/bench_{baseline,current}.json` 增加最小字段：
- `tests.intents.world_id`
- `snapshots.debug_metrics.world_tick_stats`（至少 1 个 world，包含 `tick_overrun_total`）
- `snapshots.debug_metrics.world_queue_stats`（包含 `pending_queue_len`）

- [ ] **Step 2: 扩展 test_diff 断言（先红）**

`scripts/benchmarks_tests/test_diff.py` 增加：
```python
self.assertIn("tick_overrun_total_sum:", md)
self.assertIn("bench_world_pending_queue_len:", md)
```

- [ ] **Step 3: 运行单测确认失败（RED）**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_diff.py
```

---

## Task 2: 实现 diff 摘要（GREEN）

- [ ] **Step 1: 在 benchmarks_diff.py 增加两个提取函数**

新增：
- `sum_tick_overrun_total(d:dict)->int|None`
  - 遍历 `snapshots.debug_metrics.world_tick_stats.*.tick_overrun_total` 求和
- `bench_pending_queue_len(d:dict)->int|None`
  - `worldID = tests.intents.world_id`（优先）否则尝试从 `meta.sha` 推断 `w_bench_<sha>`
  - 取 `snapshots.debug_metrics.world_queue_stats[worldID].pending_queue_len`

- [ ] **Step 2: diff_summary 末尾追加两个章节**

输出：
```
## world_tick_stats summary

tick_overrun_total_sum: <base> → <cur>

## queue depth summary

bench_world_pending_queue_len: <base> → <cur>
```

- [ ] **Step 3: 单测转绿**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_diff.py
```

- [ ] **Step 4: Commit**
```bash
git add scripts/benchmarks_diff.py scripts/benchmarks_tests/test_diff.py scripts/benchmarks_samples/bench_*.json
git commit -m "feat(bench): diff adds tick+queue summaries"
```

---

## Task 3: 回归与两次冒烟验证

- [ ] **Step 1: python unittest 全量**
```bash
python3 -m unittest -v
```

- [ ] **Step 2: 起服务 + 连跑两次 benchmarks_v2.sh**
```bash
go run ./cmd/server
BASE_URL=http://localhost:8080 bash ./scripts/benchmarks_v2.sh
BASE_URL=http://localhost:8080 bash ./scripts/benchmarks_v2.sh
```

检查第二次生成的 `.diff.md`：
- 包含 `tick_overrun_total_sum:`
- 包含 `bench_world_pending_queue_len:`

- [ ] **Step 3: go test**
```bash
go test ./...
```

- [ ] **Step 4: 提交新快照（可选但推荐）**
```bash
git add docs/ops/benchmarks/*.json docs/ops/benchmarks/*.md docs/ops/benchmarks/*.diff.md
git commit -m "docs(ops): add benchmarks v2.2 snapshots"
```

- [ ] **Step 5: 覆盖包**
```bash
git format-patch -5 -o /workspace/patches_benchmarks_v2_2
cd /workspace && zip -qr patches_benchmarks_v2_2.zip patches_benchmarks_v2_2
```

