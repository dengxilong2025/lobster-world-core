# Benchmarks v2.3（agent_batch）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 v0.2-M2 批测通道纳入 benchmarks v2：新增独立 agent_bench（产出 agent_batch JSON），benchmarks_v2.sh 合并进入 `tests.agent_batch`，并在 diff/md 中展示与判回归。

**Architecture:** 新增 `scripts/benchmarks_agent_v0_2_m2.py` 负责运行 `agent_test_v0_2_m2.sh` 并输出摘要 JSON；benchmarks_v2.sh 捕获该 JSON 并写入主 benchmarks.json；benchmarks_diff.py 增加 `agent_batch` 表格 diff；补齐 python 单测门禁（samples + test_diff）。

**Tech Stack:** python3（标准库 json/subprocess/unittest）、bash。

---

## 0) Files（锁定）

**Create:**
- `scripts/benchmarks_agent_v0_2_m2.py`

**Modify:**
- `scripts/benchmarks_v2.sh`
- `scripts/benchmarks_diff.py`
- `scripts/benchmarks_tests/test_diff.py`
- `scripts/benchmarks_samples/bench_baseline.json`
- `scripts/benchmarks_samples/bench_current.json`

---

## Task 1: TDD — diff 门禁（RED）

- [ ] **Step 1: 扩展 samples：加入 tests.agent_batch**

在 `scripts/benchmarks_samples/bench_{baseline,current}.json` 增加：
```json
"agent_batch": {
  "duration_sec": 10,
  "fail_total": 0,
  "export_lines_total": 100,
  "export_bytes_total": 10000
}
```

- [ ] **Step 2: 扩展 test_diff 断言（先红）**

在 `scripts/benchmarks_tests/test_diff.py` 增加：
```python
self.assertIn("## agent_batch", md)
self.assertIn("| duration_sec |", md)
self.assertIn("| fail_total |", md)
```

- [ ] **Step 3: 运行单测确认失败（RED）**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_diff.py
```

- [ ] **Step 4: Commit（仅测试）**
```bash
git add scripts/benchmarks_samples/bench_*.json scripts/benchmarks_tests/test_diff.py
git commit -m "test(bench): gate agent_batch diff section"
```

---

## Task 2: 实现 diff：新增 agent_batch 表格（GREEN）

- [ ] **Step 1: benchmarks_diff.py 新增 agent_batch 章节**

读取：
`current["tests"]["agent_batch"]` 和 `baseline["tests"]["agent_batch"]`

输出一张表（示例）：
| metric | baseline | current | delta | verdict |

判定：
- `duration_sec`：increase 超过阈值 → REGRESSION
- `fail_total`：cur > base → REGRESSION（无视阈值）

其它字段只展示：
- `export_lines_total`
- `export_bytes_total`

- [ ] **Step 2: 单测转绿**
```bash
python3 -m unittest -v scripts/benchmarks_tests/test_diff.py
```

- [ ] **Step 3: Commit（diff 实现）**
```bash
git add scripts/benchmarks_diff.py
git commit -m "feat(bench): add agent_batch diff table"
```

---

## Task 3: 实现 agent_bench 与 benchmarks_v2.sh 合并

- [ ] **Step 1: 新增 scripts/benchmarks_agent_v0_2_m2.py**

行为：
- `subprocess.run(["bash","scripts/agent_test_v0_2_m2.sh", ...])`
- 找到最新 `out/agent_runs/<ts>/summary.json`（用脚本 stdout 里的 run_dir 更稳）
- 输出精简 JSON 到 stdout，至少包含：
  - `duration_sec`
  - `fail_total`
  - `export_lines_total`
  - `export_bytes_total`
  - `world_id` / `n`

- [ ] **Step 2: benchmarks_v2.sh 合并**

在构建主 json 的 python block 里：
- 若传入了 `agent_json` 路径且存在：
  - `out["tests"]["agent_batch"] = json.loads(read(agent_json))`

并在 md 生成里增加 `## agent_batch` 小节。

- [ ] **Step 3: 回归**
```bash
python3 -m unittest -v
```

- [ ] **Step 4: Commit（agent_bench + merge）**
```bash
git add scripts/benchmarks_agent_v0_2_m2.py scripts/benchmarks_v2.sh
git commit -m "feat(bench): integrate v0.2-M2 agent batch into benchmarks"
```

---

## Task 4: 冒烟与交付

- [ ] **Step 1: 起服务 + 跑一次 benchmarks_v2.sh**
```bash
go run ./cmd/server
BASE_URL=http://localhost:8080 bash ./scripts/benchmarks_v2.sh
```
检查生成的 `.json/.md/.diff.md` 中包含 `agent_batch`。

- [ ] **Step 2: go test**
```bash
go test ./...
```

- [ ] **Step 3: push**
```bash
git push origin main
```

- [ ] **Step 4: 覆盖包 & 项目包**
```bash
git format-patch -6 -o /workspace/patches_benchmarks_v2_3
cd /workspace && zip -qr patches_benchmarks_v2_3.zip patches_benchmarks_v2_3
cd /workspace && zip -qr lobster-world-core-project-latest.zip lobster-world-core-git -x "lobster-world-core-git/.git/*" -x "lobster-world-core-git/out/*"
```

