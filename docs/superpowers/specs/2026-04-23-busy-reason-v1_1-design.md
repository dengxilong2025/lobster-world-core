# BUSY reason v1.1（可复现测试闭环）Design

**目标**：在已有 `busy_by_reason` 的基础上，把三类原因（`intent_ch_full / pending_queue_full / accept_timeout`）做成 **稳定可复现、可测试、可回归** 的闭环，并补齐 benchmarks 输出摘要。

**范围**：
- sim：新增 `IntentChannelCap` 可配置（允许 0=unbuffered）以稳定复现 `intent_ch_full`
- tests：新增/补齐 integration tests 覆盖 `intent_ch_full` 与 `accept_timeout`
- scripts：`benchmarks_run.sh` 增加 `busy_by_reason` 摘要段（不依赖 jq）

**非目标**：
- 不引入 Prometheus/k6
- 不改变事件 schema / replay 决定论

---

## 1) 现状与缺口

现状：
- sim 已返回 `BusyError{Reason}` 并保持 `errors.Is(err, ErrBusy)` 兼容
- gateway 已映射任意 `errors.Is(err, sim.ErrBusy)` 为 `503 BUSY`，并计数 `busy_by_reason.*`

缺口：
- `intent_ch_full` 在默认 `intentCh cap=256` 下不稳定（很难在测试中稳定填满/复现）
- `accept_timeout` 当前虽会映射成 503，但缺少集成门禁测试
- benchmarks 报告目前只有完整 metrics JSON，没有“摘要段”

---

## 2) 设计

### 2.1 sim：IntentChannelCap（允许 0）

新增：
- `sim.Options.IntentChannelCap *int`
  - `nil` => 默认 256（保持现状）
  - `0` => unbuffered（用于稳定复现 `intent_ch_full`）
  - `>0` => buffered cap

实现：
- `Engine` 增加 `intentChannelCap int` 字段
- `Engine.New`：若 `opts.IntentChannelCap!=nil` 使用其值，否则默认 256
- `newWorld` 增加参数 `intentChCap int`，并用 `make(chan queuedIntent, intentChCap)`

### 2.2 可复现测试策略

#### intent_ch_full（稳定复现）

手段：
1) 配置 `IntentChannelCap=0`（unbuffered）
2) 使用一个会阻塞 `Append` 的 EventStore，使 world goroutine 卡在 `handleIntent -> appendAndPublish -> Append`
3) 当 world goroutine 卡住时，它不再处于 select 接收 intent 的状态；此时第二次 SubmitIntent 因 channel 无 receiver 而触发 `BusyReasonIntentChFull`

#### accept_timeout（稳定复现）

手段：
1) EventStore.Append 阻塞
2) `IntentAcceptTimeout=10ms`
3) SubmitIntent 触发 `BusyReasonAcceptTimeout`
4) gateway 映射为 `503 BUSY` 并计入 `busy_by_reason.accept_timeout`

### 2.3 benchmarks 输出摘要

在 `benchmarks_run.sh` 中新增：
- “busy_by_reason 摘要”段：用 `python3 -c` 从 `/debug/metrics` JSON 中提取 `metrics.busy_by_reason` 并输出

---

## 3) 测试门禁（integration）

新增：
1) `TestBusyReasonMetrics_IntentChFull`：断言 `busy_by_reason.intent_ch_full >= 1`
2) `TestBusyReasonMetrics_AcceptTimeout`：断言 `busy_by_reason.accept_timeout >= 1`

> 现有 `pending_queue_full` 测试保持不变。

