# 压测体系 v2.1（diff 更靠谱）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 v2 基础上增强“对比上一次”可靠性：防覆盖命名、可预测 baseline 选择、diff 表格化并覆盖更多关键指标摘要。

**Architecture:** 新增 `benchmarks_baseline.py` 负责 baseline 选择；增强 `benchmarks_diff.py` 输出 markdown 表格并带 REGRESSION 判定；改造 `benchmarks_v2.sh`：生成唯一 basename（必要时追加 `_HHMMSS`），并在 diff 中写清 current/baseline 文件名。

**Tech Stack:** bash、python3（标准库 json/re/unittest）。

---

## 0) Files（锁定）

**Create:**
- `scripts/benchmarks_baseline.py`
- `scripts/benchmarks_tests/test_baseline.py`

**Modify:**
- `scripts/benchmarks_v2.sh`
- `scripts/benchmarks_diff.py`
- `scripts/benchmarks_tests/test_diff.py`（增强表格断言）

---

## Task 1: TDD — baseline 选择（RED→GREEN）

**Files:**
- Create: `scripts/benchmarks_tests/test_baseline.py`
- Create: `scripts/benchmarks_baseline.py`

- [ ] **Step 1: 写 failing test（先红）**

`scripts/benchmarks_tests/test_baseline.py`：
```python
import unittest
from scripts.benchmarks_baseline import pick_baseline

class TestBaseline(unittest.TestCase):
    def test_pick_latest_not_current(self):
        files = [
            "docs/ops/benchmarks/2026-04-23_aaaaaaa.json",
            "docs/ops/benchmarks/2026-04-23_bbbbbbb.json",
            "docs/ops/benchmarks/2026-04-23_ccccccc.json",
        ]
        cur = "docs/ops/benchmarks/2026-04-23_ccccccc.json"
        self.assertEqual(pick_baseline(files, cur), "docs/ops/benchmarks/2026-04-23_bbbbbbb.json")
```

- [ ] **Step 2: 运行确认失败（RED）**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_baseline.py
```

- [ ] **Step 3: 最小实现 pick_baseline（转绿）**

`scripts/benchmarks_baseline.py`：
- `pick_baseline(files:list[str], current:str)->str|None`
- 按文件名排序，取“最后一个 != current”

提供 CLI：
```bash
python3 scripts/benchmarks_baseline.py --out-dir docs/ops/benchmarks --current <path>
```

- [ ] **Step 4: 单测转绿**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_baseline.py
```

- [ ] **Step 5: Commit**
```bash
git add scripts/benchmarks_baseline.py scripts/benchmarks_tests/test_baseline.py
git commit -m "test(bench): add baseline selector gates"
```

---

## Task 2: TDD — diff 表格化（RED→GREEN）

**Files:**
- Modify: `scripts/benchmarks_tests/test_diff.py`
- Modify: `scripts/benchmarks_diff.py`

- [ ] **Step 1: 增强 test_diff，断言表格出现**

```python
self.assertIn("| metric | baseline | current | delta | verdict |", md)
self.assertIn("| qps |", md)
```

- [ ] **Step 2: 运行确认失败（RED）**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_diff.py
```

- [ ] **Step 3: 修改 diff_summary 输出表格（转绿）**

实现细节：
- 每个测试一张表：
  - qps / avg_time_sec：带 Δ% 与 OK/REGRESSION
  - busy_503 / status_counts(200/429/503)：只展示不判定
- diff 头部写明：
  - current file / baseline file（由调用方传入或写入 md 头）

- [ ] **Step 4: 单测转绿**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_diff.py
```

- [ ] **Step 5: Commit**
```bash
git add scripts/benchmarks_diff.py scripts/benchmarks_tests/test_diff.py
git commit -m "feat(bench): table diff output"
```

---

## Task 3: benchmarks_v2.sh 改造（防覆盖 + baseline）

**Files:**
- Modify: `scripts/benchmarks_v2.sh`

- [ ] **Step 1: 防覆盖 basename**
- 若 `${base}.json` 已存在，追加 `_HHMMSS`。

- [ ] **Step 2: baseline 选择改用 benchmarks_baseline.py**
- 在写 current 之前选 baseline（现有逻辑保留，但换成 python 选择器）。
- diff 输出写清楚 baseline/current。

- [ ] **Step 3: Commit**
```bash
git add scripts/benchmarks_v2.sh
git commit -m "feat(bench): robust baseline selection + non-overwrite naming"
```

---

## Task 4: 回归与冒烟（两次运行验证 diff）

- [ ] **Step 1: python unittest**
```bash
python3 -m unittest -v
```

- [ ] **Step 2: 起服务并连续跑两次**
```bash
go run ./cmd/server
BASE_URL=http://localhost:8080 bash ./scripts/benchmarks_v2.sh
BASE_URL=http://localhost:8080 bash ./scripts/benchmarks_v2.sh
ls -1 docs/ops/benchmarks/*.json | sort | tail -n 5
```

检查第二次生成的 `.diff.md`：
- baseline 不为 “no baseline found”
- 表格存在

- [ ] **Step 3: go test**
```bash
go test ./...
```

- [ ] **Step 4: Commit（加入新快照）**
```bash
git add docs/ops/benchmarks/*.json docs/ops/benchmarks/*.md docs/ops/benchmarks/*.diff.md
git commit -m "docs(ops): add benchmarks v2.1 snapshots"
```

- [ ] **Step 5: format-patch 覆盖包**
```bash
git format-patch -6 -o /workspace/patches_benchmarks_v2_1
cd /workspace && zip -qr patches_benchmarks_v2_1.zip patches_benchmarks_v2_1
```

