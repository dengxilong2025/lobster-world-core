# 给“小蓝/Claude AI”的代码审计指南（v0）

> 目的：减少你手工复制代码的成本；让审计者从“入口文档 + 打包材料”快速理解上下文。

## 1) 建议的审计输入（最少集合）
- `llms.txt`（总览）
- `docs/architecture.md`（主线架构与不变式）
- `docs/roadmap.md`（阶段目标/边界）
- `docs/testing.md`（如何验证）
- 代码目录：
  - `internal/events/**`
  - `internal/sim/**`
  - `internal/projections/**`
  - `internal/gateway/**`
  - `tests/integration/**`

（v0.2-M2 推荐）补充运行证据：
- `out/agent_runs/<ts>/summary.json` + `export_*.ndjson`（批测产物，便于核对“可回放/可解释/错误码语义稳定”）
- `GET /api/v0/debug/config` 输出（运行时关键开关快照，用于排障与审计）

## 2) 审计重点（按优先级）
### A. 不偏离初心（架构一致性）
- 是否仍以事件日志为唯一真相？是否引入了“第二真相”存储？
- Projection 是否可丢弃并从 EventStore 重建？
- Replay 是否仍能用同一套事件重放？

### B. 决定论与可回放
- 是否使用 wall clock 决策（time.Now 参与核心逻辑）？
- 随机数是否全部 seed 化？
- Tick 与 Ts 是否一致（tick-based scoring、ts 的严格单调性）？

### C. 长期运行的资源边界
- 任何 map/slice 是否会无限增长（epochChoice、rate limiter、projection cache）？
- goroutine 是否可退出（Engine.Stop、Hub subscribe/unsub）？

### D. 对外 API 稳定性
- 错误码与语义是否稳定（429 RATE_LIMITED、503 BUSY 等）？
- Validate 规则是否一致（actors 必填、ts 必填等）

## 3) 输出格式建议
- “必须修复（P0）/建议修复（P1）/可优化（P2）”
- 每条问题包含：复现路径/受影响范围/建议改法/风险说明
