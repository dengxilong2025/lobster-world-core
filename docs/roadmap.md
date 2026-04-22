# 路线图（滚动更新）

> 目标：在不偏离“事件溯源 + 投影 + 仿真”的初心前提下，分阶段把“世界可运行、可观战、可回放、可扩展”落地。

## 阶段 0/1/2（已完成）
- [x] 事件规范/存储/基础查询
- [x] world state clamp（安全值域）
- [x] spectator projection 实时订阅（Hub）
- [x] shock epochChoice 有界缓存
- [x] 默认 world 常量中心化
- [x] EventStore GetByID（二级索引）
- [x] auth IP 限速
- [x] intent 队列背压（503 BUSY）
- [x] spectator hotness halfLife 改为 tick-based
- [x] shock 配置从 env 读取（默认关闭）
- [x] gateway 路由模块化拆分

## 阶段 3（世界活起来：进行中）
- [x] v0 intent 规则集（Goal -> Delta）
- [x] idle-time 内生演化（world_evolved）
- [x] Trace 自动填充（意图执行/演化）
- [x] replay/highlight 蝴蝶效应增强（trace + cause narrative）
- [x] v0.3-M1 最小剧本层：外交/贸易事件（alliance_formed / treaty_signed / trade_agreement）
- [x] spectator 首页“世界阶段/状态摘要”：行动建议优先（关键词+预期事件类型）
- [x] 更稳定的“冲击期脚本化回放”（shock 生命周期 beats 结构更一致）

## 阶段 4（工程化/上线准备：待定）
- [x] 指标与可观测性（debug/metrics：请求总量、状态码分布、BUSY 计数）
- [ ] 压测脚本与基准（auth/intents/sse）
- [x] replay/export 的稳定格式与版本化
