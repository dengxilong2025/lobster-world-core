# 版本质量门槛（QA Gates）与第三方测试流程

> 目标：每个版本发布前，都走同一套“最低质量门槛”，避免偏离路线与架构腐化。

## 1) Gate-0：基础可构建/可测试
- [ ] `go test ./...` 全绿
- [ ] CI（若启用）通过
- [ ] 无明显 flaky（至少连续跑 3 次不失败）

## 2) Gate-1：核心业务不变式
- [ ] spec.Event.Validate 不变式不被破坏（actors/ts/world_id/type/narrative 必填）
- [ ] WorldState clamp 不变式仍然成立
- [ ] 背压/限速语义稳定（429/503 等）

## 3) Gate-2：决定论与回放
- [ ] determinism 集成测试通过（同 seed 同输入 → 事件序列与状态一致）
- [ ] replay/export 可用于离线复现（NDJSON）
- [ ] replay/highlight 输出结构向后兼容（只增字段/beat，不破坏旧字段）

## 4) Gate-3：体验与可读性（观战/回放）
- [ ] spectator/home 能给出“世界阶段/近期/风险(或建议)”的可读摘要
- [ ] replay/highlight 至少包含：事件本体 + 世界阶段 + 近期摘要 + 1 条风险/建议

## 5) Gate-4：第三方测试（Claude/小蓝）

### 输入材料（推荐）
- `review_bundle.zip`（一键打包，含 docs+code+tests）
- 或至少：`llms.txt` + `docs/architecture.md` + `docs/testing.md` + 关键代码目录

此外（v0.2-M2 推荐提供）：
- `out/agent_runs/<ts>/summary.json`：一轮批测的成功/失败计数与失败原因聚合
- `out/agent_runs/<ts>/export_*.ndjson`：对应世界的事件导出（可离线复现/对比）

### 输出格式（统一）
每轮第三方测试输出一个 Markdown：
- P0（必须修复）：会导致崩溃/数据错/安全风险/决定论破坏
- P1（建议修复）：高风险维护点、资源泄漏、可观测性缺失
- P2（可优化）：可读性/性能/结构

### 合格判定
- P0 必须清零
- P1 有明确计划与排期（写入 WBS）
