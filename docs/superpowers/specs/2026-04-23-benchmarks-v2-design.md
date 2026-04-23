# 压测体系 v2（benchmarks 归档 + 自动对比）Design

## 目标

把现有 `scripts/benchmarks_run.sh` 从“生成一份 Markdown 记录”升级为“可回归基准”：

1) **结构化摘要**：每次压测产生一份 `*.json`（机器可读），并生成对应 `*.md`（人类可读）。
2) **自动对比上一次**：默认对比 `docs/ops/benchmarks/` 目录下“最近一份”历史快照，输出 diff 摘要并标记回归。
3) **低成本、可移植**：继续使用 bash+curl+python3（标准库），不引入外部依赖，不放入 CI。

## 已确认的产品决策

- 默认对比基线：**比上一次快照**（最近一份）
- 回归阈值：**10%**（QPS 下降或 AVG_TIME 上升超过 10% 标记 REGRESSION）

---

## 1) 现状与痛点

现状（v1）：
- `scripts/benchmarks_run.sh` 生成 `docs/ops/benchmarks/YYYY-MM-DD_<sha>.md`
- 内容以“原始输出 + 部分 JSON 截断”为主，便于浏览，但不利于程序化对比

痛点：
- 无统一的“可对比字段”（QPS/AVG/503 等需要从文本里猜）
- 无自动 diff：回归只能人工翻两份 md
- metrics/config 快照可能被截断，难以做稳定提取

---

## 2) 产物与目录结构

每次运行生成一组同名文件（同一 basename）：

```
docs/ops/benchmarks/
  YYYY-MM-DD_<sha>.md          # 人类可读报告
  YYYY-MM-DD_<sha>.json        # 机器可读摘要（核心）
  YYYY-MM-DD_<sha>.diff.md     # 与上一次快照的对比摘要（可选生成；默认生成）
```

说明：
- `.json` 是 v2 的“真源数据”（source of truth）
- `.md`/`.diff.md` 都可由 `.json` 重新生成（未来可选）

---

## 3) JSON 摘要 schema（v1）

文件：`docs/ops/benchmarks/YYYY-MM-DD_<sha>.json`

```json
{
  "meta": {
    "date": "2026-04-23",
    "sha": "abcdef1",
    "base_url": "http://localhost:8080",
    "go_version": "go version go1.22.6 linux/amd64",
    "uname": "Linux ...",
    "threshold_regression_pct": 10
  },
  "tests": {
    "auth_challenge": {
      "duration_sec": 0.0,
      "total": 200,
      "qps": 123.4,
      "avg_time_sec": 0.0012,
      "status_counts": {"200": 200},
      "busy_503": 0
    },
    "intents": { "...": "同上结构（额外记录 world_id）" },
    "replay_export": { "...": "同上结构（额外记录 avg_bytes）" }
  },
  "snapshots": {
    "debug_config": { "...": "原样 JSON（不截断）" },
    "debug_metrics": {
      "busy_by_reason": {"intent_ch_full": 0, "pending_queue_full": 0, "accept_timeout": 0},
      "world_queue_stats": { "...": "原样（低基数）" },
      "world_tick_stats": { "...": "原样（低基数）" }
    }
  }
}
```

设计取舍：
- `debug_metrics` 只保留“低基数、可对比”的子树（避免把整棵 metrics 大对象塞进 json）
- `status_counts` 统一为字符串 key（JSON map）

---

## 4) 自动对比（diff）规则

### 4.1 基线选择

默认基线：`docs/ops/benchmarks/` 下最近一份 `*.json`（按文件名排序，取倒数第二份或显式挑选最近非当前 sha 的文件）。

若目录为空或找不到基线：
- 仍生成本次 `.json` 与 `.md`
- `.diff.md` 输出 “no baseline found”

### 4.2 对比指标

对每个 test（auth_challenge/intents/replay_export）输出：
- QPS：baseline vs current，Δ%、是否回归（下降超过 10%）
- AVG_TIME_SEC：baseline vs current，Δ%、是否回归（上升超过 10%）
- BUSY_503：baseline vs current（仅展示差异，不做阈值判断）
- STATUS_COUNTS：展示 200/429/503（存在则展示）

对关键 metrics 输出：
- `busy_by_reason`：逐 key 展示 baseline vs current（差异）
- `world_tick_stats`：只展示聚合摘要（例如所有 world 的 `tick_overrun_total` 总和；或只展示默认 bench world）

---

## 5) 实现方案（推荐）

### 5.1 方案 A（推荐）：bash 负责运行，python3 负责解析/生成

新增脚本/模块：
- `scripts/benchmarks_run.sh`：负责运行 loadtest + 拉取 debug/config/metrics（复用现有）
- `scripts/benchmarks_parse.py`：把 loadtest 输出文本解析成结构化 dict
- `scripts/benchmarks_write_report.py`：从 `*.json` 生成 `*.md`（可选；v2 初期也可以仍由 bash 直接输出 md）
- `scripts/benchmarks_diff.py`：读取 current+baseline 两份 json，输出 `.diff.md`

优点：
- python3 标准库足够（json/re/argparse），解析更稳
- bash 保持轻量，便于在不同环境直接跑

缺点：
- 脚本文件会多一些，但职责清晰、可测试（用固定样例输入）

### 5.2 方案 B：Go 实现一个 `cmd/bench` 工具

优点：类型安全、解析更强
缺点：工程成本更高；不符合“脚本轻量”的当前节奏

结论：选方案 A。

---

## 6) 测试/门禁

不把 benchmarks 放 CI，但需要“可自测门禁”：

- 为 `benchmarks_parse.py` 增加样例输入输出单测（python `unittest`）：
  - 输入：保存一份 loadtest 输出样例（小文件）
  - 输出：断言能提取 `TOTAL/QPS/AVG_TIME/STATUS_COUNTS/BUSY_503`
- 为 `benchmarks_diff.py` 增加样例 json 对比单测：
  - 断言 10% 阈值触发时会标记 `REGRESSION`

---

## 7) 向后兼容与迁移

- v2 上线后，旧的 `*.md` 仍保留；新生成会同时产生 `*.md + *.json (+ *.diff.md)`
- 未来如需统一历史数据，可选补一个一次性迁移脚本从旧 md 近似抽取（不在 v2 范围）

