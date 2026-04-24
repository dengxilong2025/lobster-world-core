# Benchmarks v2.4（Verdict breakdown）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 强化 `benchmarks_diff.py` 的 `## Verdict`：输出回归条目清单与计数，明确回归来源（含 agent_batch）。

**Architecture:** 在 diff 生成过程中收集 `regressions[]`（字符串行）；在 Verdict 中输出 `REGRESSION detected (N).` + 条目列表；扩展 python 单测门禁确保输出稳定。

**Tech Stack:** python3 unittest。

---

## 0) Files（锁定）

**Modify:**
- `scripts/benchmarks_diff.py`
- `scripts/benchmarks_tests/test_diff.py`

---

## Task 1: TDD — Verdict 门禁（RED）

- [ ] **Step 1: 扩展 test_diff 断言（先红）**

在 `scripts/benchmarks_tests/test_diff.py` 追加断言：
```python
self.assertIn("REGRESSION detected (", md)
self.assertIn("- intents.avg_time_sec:", md)
self.assertIn("- agent_batch.duration_sec:", md)
```

- [ ] **Step 2: 运行确认失败（RED）**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_diff.py
```

- [ ] **Step 3: Commit（仅测试）**
```bash
git add scripts/benchmarks_tests/test_diff.py
git commit -m "test(bench): gate verdict breakdown"
```

---

## Task 2: 实现 Verdict breakdown（GREEN）

- [ ] **Step 1: benchmarks_diff.py 收集 regressions**

实现方式：
- 在每次 `_metric_row(...)->(row, reg)` 返回 `reg=True` 时，将一条格式化字符串 push 到 `regressions`：
  - `{test}.{metric}: {base} → {cur} ({delta_pct})`
- 对 `agent_batch.fail_total` 的特殊规则（cur > base）同样 push：
  - `agent_batch.fail_total: 0 → 2`

- [ ] **Step 2: 输出 Verdict**

若 `regressions` 非空：
```
## Verdict

REGRESSION detected (N).

- ...
- ...
```

否则：
```
## Verdict

OK.
```

- [ ] **Step 3: 单测转绿**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_diff.py
```

- [ ] **Step 4: Commit**
```bash
git add scripts/benchmarks_diff.py
git commit -m "feat(bench): verdict breakdown list"
```

---

## Task 3: 回归与交付

- [ ] **Step 1: python unittest + go test**
```bash
python3 -m unittest -v
go test ./...
```

- [ ] **Step 2: 冒烟跑一轮 benchmarks_v2.sh**
```bash
go run ./cmd/server
BASE_URL=http://localhost:8080 bash ./scripts/benchmarks_v2.sh
```
检查最新 `.diff.md` 的 `## Verdict` 是否含计数与条目列表。

- [ ] **Step 3: push + 覆盖包/项目包**
```bash
git push origin main
git format-patch -4 -o /workspace/patches_benchmarks_v2_4
cd /workspace && zip -qr patches_benchmarks_v2_4.zip patches_benchmarks_v2_4
cd /workspace && zip -qr lobster-world-core-project-latest.zip lobster-world-core-git -x "lobster-world-core-git/.git/*" -x "lobster-world-core-git/out/*"
```

