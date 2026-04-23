# 可观测性 v2（队列背压口径）Design

**目标**：在现有 debug/metrics v1 基础上，新增一组“解释 BUSY/吞吐/响应变慢”的 **队列背压指标**，优先让我们能回答：

1) 现在是不是排队了？排多少？
2) SubmitIntent 从提交到被 **durably accepted**（写入 `intent_accepted`）平均要等多久？
3) BUSY 主要发生在“入口 channel 满”还是“已接受但待执行队列满”？

**范围**：
- 扩展 `GET /api/v0/debug/metrics`（只新增字段，向后兼容）
- sim 增加一个只读的 QueueStats 暴露（不改变事件 schema、不改变 replay）
- gateway/intents 增加 submit 等待耗时的计数（wall-clock，但只用于 metrics，不影响 sim 决定论）

---

## 1) 背景：当前背压实际发生的位置

per-world 有两道闸：

1) `world.intentCh`（buffer=256）
   - `world.submitIntent` 往 channel 写入，满则 **直接返回 ErrBusy**（未进入世界 loop）
2) `world.queue []queuedIntent`（maxQueue = MaxIntentQueue）
   - `world.handleIntent` 持久化 `intent_accepted` 后才会 append 到 `w.queue`
   - 当 `len(w.queue) >= w.maxQueue`，会对本次 intent 的 ack 返回 **ErrBusy**

因此，队列背压观测需要同时覆盖：
- channel 的瞬时拥塞（入口 burst）
- pending queue 的持续拥塞（世界处理不过来）

---

## 2) 新增指标（debug/metrics v2-queue）

### 2.1 Submit 等待耗时（从请求到 accepted）

**新增：**
- `intent_accept_wait_ms_total`
- `intent_accept_wait_count`

定义：
- 在 gateway `/api/v0/intents` handler 中，围绕 `sm.SubmitIntent` 做 wall-clock 计时：
  - 成功（200）则记录 `wait_ms`（至少 1ms 以避免 0）
  - BUSY/INTERNAL 不计入（或分开计入 errors，总之 v2 先只统计成功的 accept wait）

用途：
- 粗略估算“系统在当前负载下，从提交到 durable acceptance 的等待成本”

### 2.2 per-world 队列深度快照（低基数）

**新增：**
- `world_queue_stats`（object map，key 为 world_id，value 为固定字段）

每个 world 的字段（全部是整数）：
- `intent_ch_len` / `intent_ch_cap`
- `pending_queue_len`（len(w.queue)）
- `pending_queue_max`（w.maxQueue）
- `tick`（w.tick，便于与状态对齐）

示例：
```json
"world_queue_stats": {
  "w1": {"intent_ch_len": 0, "intent_ch_cap": 256, "pending_queue_len": 12, "pending_queue_max": 1024, "tick": 88}
}
```

获取方式：
- 在 sim.Engine 新增 `QueueStats()`，返回 `map[string]QueueStat`
- `QueueStat` 由 world 内部锁保护读取（只读快照）
- debug/metrics handler 在返回 JSON 时，把该快照合并进 metrics（无需做 atomic）

### 2.3 BUSY 原因拆分（可选 v2.1）

> v2 主线先不强制做（改动较大），但保留扩展点。

可选新增：
- `busy_by_reason`：`{"intent_ch_full": X, "pending_queue_full": Y}`

实现方式（建议 v2.1 再做）：
- sim 定义 `type BusyError struct{ Reason string }` 并 `errors.Is(err, ErrBusy)=true`
- gateway 根据 error reason 打点

---

## 3) 测试门禁（integration）

新增集成测试覆盖：
1) `intent_accept_wait_ms_total/count` 会增长（提交 3 个 intents 后 count>=3）
2) 制造背压后 `world_queue_stats[worldID].pending_queue_len > 0`
   - 方式：把 `TickInterval` 设大（例如 5s）让队列不易 drain，快速提交多条 intent
3) 在停止提交并等待少量时间后（或手动推进 tick），`pending_queue_len` 能下降（非强制为 0，避免 flaky）

---

## 4) 风险与控制

- `world_queue_stats` 是 debug 视角：世界数多时 JSON 会变大，但目前 world_id 数量低（低基数），可接受
- `intent_accept_wait_*` 使用 wall-clock，但仅用于观测，不参与 sim 决策；保证决定论不受影响

