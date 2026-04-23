# 压测体系 v2（benchmarks 归档 + 自动对比）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `benchmarks_run.sh` 升级为 v2：每次运行生成 `.json/.md/.diff.md` 三件套，默认与“上一次快照”对比，并以 10% 阈值标记 REGRESSION。

**Architecture:** bash 负责运行与拉取 debug 快照；python3（标准库）负责解析 loadtest 输出、写入结构化 json、生成 markdown 报告与 diff 报告。`.json` 为真源数据，`.md/.diff.md` 从 json 生成。

**Tech Stack:** bash、curl、python3（json/re/argparse/unittest）。

---

## 0) Files（锁定）

**Create:**
- `scripts/benchmarks_parse.py`（解析 loadtest 输出 → dict）
- `scripts/benchmarks_diff.py`（current vs baseline → diff markdown）
- `scripts/benchmarks_v2.sh`（编排：跑 loadtest、拉取快照、生成 json+md+diff）
- `scripts/benchmarks_samples/loadtest_auth_challenge.txt`
- `scripts/benchmarks_samples/loadtest_intents.txt`
- `scripts/benchmarks_samples/loadtest_replay_export.txt`
- `scripts/benchmarks_samples/bench_current.json`
- `scripts/benchmarks_samples/bench_baseline.json`
- `scripts/benchmarks_tests/test_parse.py`
- `scripts/benchmarks_tests/test_diff.py`

**Modify:**
- `docs/ops/benchmarks/.gitkeep`（保留）

**(Optional) Keep:**
- `scripts/benchmarks_run.sh`（v1 保留；v2 用新的 `benchmarks_v2.sh`，避免破坏已有流程）

---

## Task 1: TDD — 解析器单测（RED→GREEN）

### 1.1 解析样例准备

- [ ] **Step 1: 把现有脚本输出保存为样例**

手工保存三段典型输出（来自现有脚本的 stdout），放到：
- `scripts/benchmarks_samples/loadtest_auth_challenge.txt`
- `scripts/benchmarks_samples/loadtest_intents.txt`
- `scripts/benchmarks_samples/loadtest_replay_export.txt`

要求覆盖字段（用真实输出即可）：
- `DURATION_SEC`
- `TOTAL`
- `QPS≈...`
- `AVG_TIME_SEC=...`
- `STATUS_COUNTS` 段
- `BUSY_503`（intents）
- `AVG_BYTES`（replay_export）

### 1.2 写 failing 单测

- [ ] **Step 2: 新增 `scripts/benchmarks_tests/test_parse.py`（先失败）**

```python
import unittest
from pathlib import Path

from scripts.benchmarks_parse import parse_loadtest_output

class TestParse(unittest.TestCase):
    def test_parse_auth(self):
        txt = Path("scripts/benchmarks_samples/loadtest_auth_challenge.txt").read_text(encoding="utf-8")
        out = parse_loadtest_output(txt)
        self.assertIn("qps", out)
        self.assertIn("avg_time_sec", out)
        self.assertIn("status_counts", out)

    def test_parse_intents_has_busy(self):
        txt = Path("scripts/benchmarks_samples/loadtest_intents.txt").read_text(encoding="utf-8")
        out = parse_loadtest_output(txt)
        self.assertIn("busy_503", out)

    def test_parse_export_has_avg_bytes(self):
        txt = Path("scripts/benchmarks_samples/loadtest_replay_export.txt").read_text(encoding="utf-8")
        out = parse_loadtest_output(txt)
        self.assertIn("avg_bytes", out)

if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 3: 运行单测确认失败**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_parse.py
```

### 1.3 最小实现解析器

- [ ] **Step 4: 实现 `scripts/benchmarks_parse.py`（转绿）**

要求：
- 提供 `parse_loadtest_output(text:str)->dict`
- 使用 `re` 提取：
  - duration_sec/total/qps/avg_time_sec/avg_bytes/busy_503
  - status_counts（提取 200/400/429/503 等 key）
- 若缺失字段，保持 key 不存在（不要造假），由调用方决定兜底

- [ ] **Step 5: 运行单测转绿**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_parse.py
```

- [ ] **Step 6: Commit**
```bash
git add scripts/benchmarks_parse.py scripts/benchmarks_tests/test_parse.py scripts/benchmarks_samples/*.txt
git commit -m "test(bench): add loadtest parser + samples"
```

---

## Task 2: TDD — diff 逻辑单测（RED→GREEN）

- [ ] **Step 1: 增加样例 json（baseline/current）**

在：
- `scripts/benchmarks_samples/bench_baseline.json`
- `scripts/benchmarks_samples/bench_current.json`

放入最小字段集（只需覆盖 diff 用到的字段）：
- meta.threshold_regression_pct=10
- tests.auth_challenge/intents/replay_export 的 qps/avg_time_sec/busy_503/status_counts
- snapshots.debug_metrics.busy_by_reason

- [ ] **Step 2: 写 failing 单测 `scripts/benchmarks_tests/test_diff.py`**

```python
import unittest, json
from pathlib import Path
from scripts.benchmarks_diff import diff_summary

class TestDiff(unittest.TestCase):
    def test_regression_threshold(self):
        base = json.loads(Path("scripts/benchmarks_samples/bench_baseline.json").read_text())
        cur = json.loads(Path("scripts/benchmarks_samples/bench_current.json").read_text())
        md = diff_summary(cur, base, threshold_pct=10)
        self.assertIn("REGRESSION", md)

if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 3: 运行单测确认失败**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_diff.py
```

- [ ] **Step 4: 实现 `scripts/benchmarks_diff.py`**

要求：
- `diff_summary(current:dict, baseline:dict, threshold_pct:int)->str` 返回 markdown
- 计算百分比变化：
  - QPS：`(cur-base)/base`
  - AVG_TIME_SEC：同上
- REGRESSION 规则：
  - QPS 下降超过 threshold_pct
  - AVG_TIME 上升超过 threshold_pct

- [ ] **Step 5: 单测转绿**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_diff.py
```

- [ ] **Step 6: Commit**
```bash
git add scripts/benchmarks_diff.py scripts/benchmarks_tests/test_diff.py scripts/benchmarks_samples/bench_*.json
git commit -m "test(bench): add diff logic gates"
```

---

## Task 3: v2 编排脚本（产物三件套）

**Files:**
- Create: `scripts/benchmarks_v2.sh`

- [ ] **Step 1: 写 `benchmarks_v2.sh`**

行为：
1) 读取 `BASE_URL`（默认 localhost）
2) 读取 `sha/date`，确定 basename：`docs/ops/benchmarks/YYYY-MM-DD_<sha>`
3) 跑三段 loadtest：
   - `loadtest_auth_challenge.sh`
   - `loadtest_intents.sh`（world_id 固定为 `w_bench_${sha}`）
   - `loadtest_replay_export.sh`（同 world_id）
4) 拉取 debug/config 与 debug/metrics
5) 用 python3：
   - 解析 loadtest 输出 → tests.*
   - 组合 meta + snapshots（只保留低基数 metrics 子树）
   - 写入 `${base}.json`
6) 生成 `${base}.md`（可用 python 或 bash；v2 初期允许用 python 直接输出简单 md）
7) 查找“上一次”基线 `${baseline}.json`：
   - 若存在：生成 `${base}.diff.md`
   - 若不存在：生成 diff.md 写明 no baseline

- [ ] **Step 2: Commit**
```bash
git add scripts/benchmarks_v2.sh
git commit -m "feat(bench): add benchmarks v2 runner"
```

---

## Task 4: 冒烟回归 + 覆盖包

- [ ] **Step 1: 本地起服务并跑 v2**
```bash
go run ./cmd/server
BASE_URL=http://localhost:8080 bash ./scripts/benchmarks_v2.sh
ls -lh docs/ops/benchmarks | tail -n 10
```

- [ ] **Step 2: 全量回归**
```bash
go test ./...
python3 -m unittest -v
```

- [ ] **Step 3: 将新产物加入 git 并提交**
```bash
git add docs/ops/benchmarks/*.json docs/ops/benchmarks/*.md docs/ops/benchmarks/*.diff.md
git commit -m "docs(ops): add benchmarks v2 snapshot"
```

- [ ] **Step 4: format-patch 覆盖包**
```bash
git format-patch -6 -o /workspace/patches_benchmarks_v2
cd /workspace && zip -qr patches_benchmarks_v2.zip patches_benchmarks_v2
```

