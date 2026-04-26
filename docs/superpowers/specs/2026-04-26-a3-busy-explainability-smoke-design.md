# A3：BUSY 可解释性 + smoke 失败自动抓取（Design，2026-04-26）

## 背景 / 痛点

当前 staging（Render Free）在外测/演示时，最常见的失败是：
- `503 BUSY`（世界处理队列满/accept 超时/intent channel 满）
- UI 或 API 偶发慢/超时（tick 抖动、overrun、SSE flush 等）

虽然我们已有：
- `GET /api/v0/debug/metrics`（含 `busy_by_reason`、`world_queue_stats`、`world_tick_stats`）
- `scripts/smoke_staging.sh`（轻量验收）

但仍存在两个排障痛点：
1) metrics 虽“信息全”，但缺少**人话摘要**（一眼看不出哪里在堵）
2) smoke 脚本失败时不会自动抓取 debug 信息，需要人工手动 curl 多次

---

## Goal

在不引入外部依赖、不改变业务 API 的前提下：
1) 让 `/api/v0/debug/metrics` 增加一个低成本、低基数的 **summary** 字段，快速解释 BUSY / 队列 / tick 状态
2) 增强 `scripts/smoke_staging.sh`：任一步骤失败时自动打印：
   - `GET /api/v0/debug/build`
   - `GET /api/v0/debug/metrics`（含 summary）

---

## Non-goals

- 不做 Prometheus 指标/仪表盘
- 不做高基数 label（如按 path/agent_id 细分）
- 不把 smoke 升级为压测（保持轻量、可重复）

---

## 设计

### 1) `/api/v0/debug/metrics`：新增 `metrics.summary`

#### 1.1 JSON 形状（新增字段）

在现有返回的 `metrics` 对象内新增：

```json
{
  "ok": true,
  "metrics": {
    "...": "...",
    "summary": {
      "busy": "busy_total=3 (intent_ch_full=2, pending_queue_full=1, accept_timeout=0)",
      "queue": "worlds=2 max_pending=64 max_intent_ch=32 hottest=w1 pending=64/64 intent_ch=32/32 tick=120",
      "tick": "worlds=2 overrun_total=12 jitter_avg_ms≈4.3 jitter_max_ms≈18 hottest=w1 overrun=10"
    }
  }
}
```

说明：
- `summary` 仅用于 debug（文本 + 少量可读统计），低风险
- 仍保留原始结构化字段（`busy_by_reason`、`world_queue_stats`、`world_tick_stats`），summary 只是“快速读”

#### 1.2 计算逻辑（原则）

- busy：
  - 基于 `busy_total` + `busy_by_reason` 直接拼字符串
  - 若 `busy_total==0`：`"ok"`

- queue（来自 `world_queue_stats`）：
  - `worlds`：world 数
  - 计算热点 world（hottest）：按 `(pending_queue_len/pending_queue_max, intent_ch_len/intent_ch_cap)` 取最大
  - `max_pending`/`max_intent_ch`：各世界 cap/max 的最大值（用于定位配置）
  - 若无世界：`"no_worlds"`

- tick（来自 `world_tick_stats`）：
  - overrun_total：所有世界 overrun 总和
  - jitter_avg_ms：`tick_jitter_ms_total / max(1, tick_jitter_count)` 的平均（四舍五入到 0.1）
  - jitter_max_ms：可先不做（当前结构没有 max）；如需要可后续扩展 sim 统计
  - hottest：按 `tick_overrun_total` 最大的 world
  - 若无世界：`"no_worlds"`

> 重要：summary 的实现不能引入锁竞争热点；以当前 debug 请求频率来说，O(worlds) 遍历可接受。

---

### 2) `scripts/smoke_staging.sh`：失败自动抓取 build + metrics

#### 2.1 行为

当 smoke 中任意一步失败（HTTP code 不符合预期/超时/响应体缺字段）：
- 在输出 FAIL 行之后，额外打印：
  - `--- debug/build ---` + `/api/v0/debug/build`（截断前 800 字符）
  - `--- debug/metrics ---` + `/api/v0/debug/metrics`（截断前 1600 字符）
- 然后 exit 1

#### 2.2 约束

- 仅在失败时抓取（成功路径不额外请求）
- 每个 debug endpoint 的 curl 使用相同的 `TIMEOUT_SEC`
- 输出截断避免刷屏

---

## 验收标准

1) staging：
   - `curl https://lobster-world-core.onrender.com/api/v0/debug/metrics` 返回中包含 `metrics.summary.busy/queue/tick`
2) smoke：
   - 人为制造失败（例如 BASE_URL 指向错误 host），脚本应在 FAIL 后自动打印 build+metrics（可能失败，但应打印出“抓取失败”的提示）
   - 当出现 503 BUSY 时，FAIL 输出必须带上 metrics.summary.busy，做到“一眼看到 busy_by_reason”

