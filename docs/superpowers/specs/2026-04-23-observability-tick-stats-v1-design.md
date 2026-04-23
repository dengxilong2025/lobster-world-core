# 可观测性：tick 观测 v1（wall-clock）Design

**Goal**：补齐“世界 loop 是否按期推进/是否卡顿”的观测能力。在不影响 sim 决定论的前提下，提供每个 world 的 tick wall-clock 统计，帮助线上排障：

- “tick 有没有在跑？”
- “tick 的实际间隔是否显著偏离 TickInterval（卡顿/阻塞）？”
- “最后一次 tick 发生在什么时候？”

**Non-goals**：
- 不引入 Prometheus/OTEL
- 不做高基数按事件类型拆分
- 不改变事件 schema、replay/export 语义

---

## 1) 指标定义（debug/metrics 新增字段）

新增一个低基数对象：

### `world_tick_stats`

类型：`map[string]TickStat`，key 为 `world_id`。

每个 world 的字段：
- `tick_count_total`：累计 tick 次数（从 world 创建后）
- `tick_last_unix_ms`：最后一次 tick 的 wall-clock 时间（Unix ms）
- `tick_jitter_ms_total`：累计漂移（ms）
  - 定义：`abs(actual_interval_ms - tick_interval_ms)` 的累计值
- `tick_jitter_count`：累计漂移采样次数（用于均值：total/count）
- `tick_overrun_total`：明显卡顿次数（当 `actual_interval_ms >= 2*tick_interval_ms` 计 1 次）

示例：
```json
{
  "metrics": {
    "world_tick_stats": {
      "w1": {
        "tick_count_total": 12,
        "tick_last_unix_ms": 1735000000123,
        "tick_jitter_ms_total": 18,
        "tick_jitter_count": 11,
        "tick_overrun_total": 0
      }
    }
  }
}
```

---

## 2) 实现策略（不破坏决定论）

关键原则：**wall-clock 只用于观测统计，不参与 sim 的决策与事件时间线**。

实现点：
- 在 `sim.world.loop()` 的 `case <-t.C:` 分支中更新 tick 统计：
  - `now := time.Now()`
  - `tick_count_total++`
  - `tick_last_unix_ms = now.UnixMilli()`
  - 若存在 `lastTickAt`：计算 `actual_interval_ms = now.Sub(lastTickAt).Milliseconds()`，更新 jitter 与 overrun
  - 更新 `lastTickAt = now`

数据暴露：
1) sim 新增只读 API：`Engine.TickStats() map[string]TickStat`
2) `GET /api/v0/debug/metrics` 在输出时合并 `world_tick_stats = sm.TickStats()`

并发/锁：
- `TickStat` 由 world 内部锁保护读写（与现有 state/tick 锁模型一致）
- `Engine.TickStats()` 获取 `Engine.mu` 快照 worlds，再逐个读取 world 的 tickStats（或在 Engine.mu 下调用 world 方法）

---

## 3) 测试门禁（integration）

新增 integration tests：
1) 字段存在：`world_tick_stats` key 存在（即使为空 map 也可）
2) tick 会增长：
   - 用小 TickInterval（例如 5ms）
   - `EnsureWorld(worldID)` 后等待 30~50ms
   - 读取 metrics，断言 `world_tick_stats[worldID].tick_count_total > 0`
   - 断言 `tick_last_unix_ms > 0`

---

## 4) benchmarks 输出增强（小改）

`scripts/benchmarks_run.sh` 增加：
- `world_tick_stats` 摘要（提取指定 world 或输出全部 map 的简短表示）

