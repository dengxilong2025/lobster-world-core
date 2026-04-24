# 压测体系 v2.2：diff 补齐解释性指标 Design

## Goal

在 v2.1 “表格化 diff + baseline 稳定选择”基础上，补齐两类对排障/解释回归很关键的指标摘要，让 diff 不只告诉你“慢了”，还告诉你“可能为什么慢了”：

1) **tick 卡顿**：`world_tick_stats` 聚合为 `tick_overrun_total_sum`
2) **队列背压**：bench world 的 `pending_queue_len`（来自 `world_queue_stats`）

---

## 1) 新增 diff 输出（markdown）

在 `.diff.md` 追加两个章节：

### 1.1 `world_tick_stats summary`

输出单行（base → cur）：

```
tick_overrun_total_sum: <base> → <cur>
```

聚合口径：
- 遍历 `snapshots.debug_metrics.world_tick_stats`（map[world_id]stat）
- 取每个 stat 的 `tick_overrun_total` 做求和
- 若字段不存在：n/a

### 1.2 `queue depth summary`

输出单行（base → cur）：

```
bench_world_pending_queue_len: <base> → <cur>
```

提取口径：
- bench world key：优先从 `tests.intents.world_id`（若存在；否则按约定 `w_bench_<sha>` 推断）
- 从 `snapshots.debug_metrics.world_queue_stats[worldID].pending_queue_len` 读取
- 若任一侧缺失：n/a

---

## 2) 测试门禁（python unittest）

扩展 `scripts/benchmarks_samples/bench_{baseline,current}.json`，补齐最小字段：
- `snapshots.debug_metrics.world_tick_stats`（包含至少一个 world 的 `tick_overrun_total`）
- `snapshots.debug_metrics.world_queue_stats`（包含 `pending_queue_len`）
- （可选）在 `tests.intents` 中加入 `world_id` 以便更稳定位 bench world

扩展 `scripts/benchmarks_tests/test_diff.py`：
- 断言 diff 输出包含：
  - `tick_overrun_total_sum:`
  - `bench_world_pending_queue_len:`

---

## 3) Non-goals

- 不引入更多高基数分桶（例如 per entity）
- 不引入跨机器归一化

