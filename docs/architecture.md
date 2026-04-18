# 架构概览（v0）

本项目坚持“事件溯源 + 投影 + 仿真引擎”的主线，不引入不必要的复杂中间层。

## 1. 核心数据结构：spec.Event
- 位置：`internal/events/spec/event.go`
- 原则：事件是唯一真相（canonical log），读模型可丢弃重建
- 关键必填字段：`schema_version=1`、`event_id`、`ts>0`、`world_id`、`scope`、`type`、`actors`、`narrative`

## 2. Canonical Log：EventStore
- 位置：`internal/events/store`
- 当前实现：`InMemoryEventStore`
  - 追加写（Append-only）
  - world_id 范围查询（Query）
  - 二级索引 `GetByID(world_id,event_id)`（避免线性扫描）

## 3. Live Delivery：Hub
- 位置：`internal/events/stream`
- 用途：进程内 Pub/Sub（SSE、Projection 实时更新、Sim 推送）

## 4. Simulation：Engine + world
- 位置：`internal/sim`
- 机制：
  - `SubmitIntent`：写入 `intent_accepted`，队列化等待 tick 执行
  - 每 tick 处理 1 条 intent（节流，易推理）
  - shock scheduler：按 epoch/offset/duration 产生冲击事件（可配置）
  - idle-time 演化：世界空转一段时间后产生 `world_evolved`（保持“世界在动”）
  - Trace 自动生成：为 replay/highlight 提供可解释因果链

## 5. Read Model：Spectator Projection
- 位置：`internal/projections/spectator`
- 机制：
  - `EnsureLoaded`：从 EventStore 懒加载近期事件
  - `Apply`：订阅 Hub 后实时增量更新
  - `Home`：生成 headline + hot_events（优先 tick-based half-life）
  - `Entity`：关系视图（结盟/背叛）与“为什么强/下一风险”提示

## 6. HTTP Gateway
- 位置：`internal/gateway`
- 拆分：`routes_*.go` 负责路由注册，`handler.go` 负责依赖注入
- 防打爆：
  - auth IP 限速（429）
  - intents 背压（503 BUSY）

