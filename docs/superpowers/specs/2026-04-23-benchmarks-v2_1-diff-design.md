# 压测体系 v2.1：diff 更靠谱 Design

## Goal

在 v2 基础上把 “对比上一次” 做到稳定、可读、可自动判回归：

1) **避免重跑覆盖**：同一天同 sha 多次运行能生成多份快照（不会覆盖同名文件），从而有真实 baseline。
2) **baseline 选择可预测**：明确“选哪一份当上一次”，并在 diff 里写清楚 current/baseline 的来源文件。
3) **diff 输出表格化**：关键指标一眼可看（base → cur → Δ% → OK/REGRESSION）。
4) **关键可解释指标纳入 diff**：不仅看 QPS/AVG，还看 busy/status/部分关键 metrics 摘要。

---

## 1) 现状问题

### 1.1 同 sha 重跑会覆盖文件

当前 basename：`YYYY-MM-DD_<sha>`。同一天同 commit 重跑会覆盖 `.json/.md/.diff.md`，导致：
- 无法生成第二份快照
- baseline 只能对比“更早的 sha”或“没有 baseline”

### 1.2 diff 输出不够结构化

目前 diff 主要是 bullet list，不利于快速扫描。

---

## 2) v2.1 设计

### 2.1 文件命名：加入 run suffix（防覆盖）

规则：
- 默认 basename：`YYYY-MM-DD_<sha>`
- 如果同名 `${basename}.json` 已存在，则追加 suffix：
  - 推荐：`_${HHMMSS}`（例如 `2026-04-23_abcdef1_163012.json`）
  - 备选：`_runN`

最终生成：
```
docs/ops/benchmarks/
  YYYY-MM-DD_<sha>[_HHMMSS].json
  YYYY-MM-DD_<sha>[_HHMMSS].md
  YYYY-MM-DD_<sha>[_HHMMSS].diff.md
```

### 2.2 baseline 选择（“上一次快照”）

定义：
- baseline = `out_dir` 下“最新且不是 current”的 `*.json`
- 若存在多个同 sha 文件：baseline 允许同 sha（用于衡量波动），但必须不是 current 文件本身

实现建议：
- 新增 `scripts/benchmarks_baseline.py`：
  - 输入：`--out-dir`、`--current <path>`
  - 输出：stdout 打印 baseline path（找不到则空）
  - 规则：按文件名排序（稳定）或按 mtime（可能不稳定）；推荐按文件名（`YYYY-MM-DD_sha_suffix`）排序

### 2.3 diff 输出表格（markdown）

diff 文件头部包含：
- current file / baseline file
- current sha/date / baseline sha/date
- threshold（10%）

每个 test 输出一个表格（示例）：

| metric | baseline | current | delta | verdict |
|---|---:|---:|---:|---|
| qps | 120.0 | 95.0 | -20.8% | REGRESSION |
| avg_time_sec | 0.0020 | 0.0030 | +50.0% | REGRESSION |
| busy_503 | 0 | 10 | +10 | — |
| status 200/429/503 | 500/0/0 | 490/0/10 | — | — |

判定规则（维持 v2）：
- QPS 下降超过阈值：REGRESSION
- AVG_TIME 上升超过阈值：REGRESSION

### 2.4 关键 metrics 摘要纳入 diff

1) `busy_by_reason`：逐 key 展示 `base → cur`
2) `world_tick_stats`：聚合摘要：
   - `tick_overrun_total_sum`（所有 world 的 overrun 总和）
3) bench world 的 queue 深度：
   - 从 `world_queue_stats["w_bench_<sha>"].pending_queue_len` 抽取（若存在）

---

## 3) 测试门禁（python 单测）

新增单测覆盖：
1) baseline 选择：
   - 给定候选文件列表，current 为其中之一，必须返回“上一个”
2) diff 表格输出：
   - 输出包含表头
   - 触发阈值时包含 `REGRESSION`
   - status_counts 行存在（当输入包含 status_counts）

---

## 4) 非目标

- 不做跨机器归一化（CPU/负载）
- 不做 CI gate
- 不做历史 md→json 迁移

