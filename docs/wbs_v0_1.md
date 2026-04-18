# V0.1 开发 WBS / 进度看板（滚动更新）

> 用途：把“第一版（v0.1）”的开发拆解成可跟踪的任务（WBS），并给出状态与完成度。  
> 维护方式：每次合并到 `main` 后更新本文件（建议按 commit/PR 粒度更新）。  
> 说明：进度百分比是**管理视角的估算**（按权重加权），不是代码行数/提交数。

## 0. 总览

- 当前里程碑：**v0.1（第一版可试用）**
- 当前总体完成度（估算）：**82%**
- 当前状态（摘要）：核心链路已跑通（Sim → EventStore → Projection → Spectator/Replay），并完成多项稳定性与观战/回放增强；剩余工作以“体验优化 + 工程化上线准备”为主。

### 完成度权重口径（可调整）

| 模块 | 权重 | 当前完成度 | 加权贡献 |
|---|---:|---:|---:|
| 事件规范/存储/回放 | 25% | 95% | 23.8% |
| Sim 引擎（tick/intent/shock/evolution） | 30% | 85% | 25.5% |
| Spectator/观战体验 | 20% | 85% | 17.0% |
| Gateway/API/保护措施 | 15% | 90% | 13.5% |
| 工程化（文档/测试/发布） | 10% | 60% | 6.0% |
| **合计** | **100%** |  | **82.0%** |

## 1. WBS（细节任务 + 状态）

状态枚举：`DONE` / `IN_PROGRESS` / `TODO` / `BLOCKED`

### 1.1 事件规范 / 存储 / 回放（25%）

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| ES-01 | spec.Event 规范与 Validate | 必填字段校验 + 单测 | DONE | `internal/events/spec` |
| ES-02 | InMemoryEventStore（Append/Query） | 单测覆盖 | DONE | `internal/events/store` |
| ES-03 | EventStore.GetByID 索引 | 集成/单测覆盖 | DONE | 用于 replay/highlight enrich |
| ES-04 | replay/export（NDJSON） | API 可用 + 排序稳定 | DONE | `/api/v0/replay/export` |
| ES-05 | replay/highlight（脚本回放） | 输出 beats + 测试覆盖 | DONE | 已接入 trace/cause/world 摘要 |
| ES-06 | 回放输出结构稳定性 | 关键字段向后兼容（增量增强） | IN_PROGRESS | 后续可加 schema_version/版本化 |

### 1.2 Simulation 引擎（30%）

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| SIM-01 | tick 引擎基础（world/engine） | 能持续 tick；Stop 可退出 | DONE | `internal/sim` |
| SIM-02 | 状态 clamp（安全值域） | 单测覆盖 | DONE | 防崩坏/防溢出 |
| SIM-03 | intent 处理：accepted → started → completed | 事件链完整 + 测试覆盖 | DONE | 每 tick 处理 1 条 |
| SIM-04 | intent 队列背压（上限） | 超限返回 ErrBusy/503 | DONE | 防长期运行堆积 |
| SIM-05 | shock scheduler（epoch/warn/start/end） | 决定论 + 集成测试 | DONE | 支持 betrayal/关系变化 |
| SIM-06 | shock epochChoice 有界缓存 | 无界增长问题修复 | DONE | 长期运行保护 |
| SIM-07 | intent rules（Goal→Delta） | 行为可感知 + 单测 | DONE | v0 关键词规则 |
| SIM-08 | idle-time 内生演化（world_evolved） | 空转也有叙事 + 单测 | DONE | 每 N idle ticks 触发 |
| SIM-09 | Trace 自动生成（因果链） | action/evolved 等有 trace | DONE | 供 replay 解释 |
| SIM-10 | 决定论（同 seed 同输入序列） | 集成测试 deep equality | DONE | `tests/integration/*determinism*` |
| SIM-11 | 更丰富的叙事机制（P1） | 至少 2-3 个新剧本/规则 | TODO | 后续可扩：迁徙、资源贸易等 |

### 1.3 Spectator / 观战体验（20%）

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| SP-01 | Projection：recent events cache + EnsureLoaded | 重启可恢复 | DONE | `internal/projections/spectator` |
| SP-02 | Home：headline + hot_events | 热度排序可解释 | DONE | tick-based half-life |
| SP-03 | Entity：关系/原因/提示 | 结盟/背叛可解释 | DONE | deterministic from log |
| SP-04 | world 阶段/摘要（state + recent） | `/spectator/home` 返回 `world` | DONE | stage/summary/state |
| SP-05 | 摘要去重与优选 | “近期：”无重复 A；A | DONE | spectator + replay 复用 |
| SP-06 | 更强“解说层”模板化 | 摘要更稳定更像主播口播 | IN_PROGRESS | 下一步可做“基于事件类型模板” |

### 1.4 Gateway / API / 保护措施（15%）

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| GW-01 | API v0 路由与 handler | endpoints 可用 | DONE | `internal/gateway/routes_*.go` |
| GW-02 | auth IP 限速 | 429 + 测试覆盖 | DONE | 防滥用 |
| GW-03 | intents 503 BUSY | 背压语义稳定 | DONE | 与 SIM-04 配套 |
| GW-04 | shock 配置外部化（env） | LW_SHOCK_ENABLED 等 | DONE | 默认关闭 |
| GW-05 | 路由模块化拆分 | handler 拆分为 routes | DONE | 可维护性 |

### 1.5 工程化（文档/测试/发布）（10%）

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| ENG-01 | 全量测试基线 | `go test ./...` 全绿 | DONE | 已作为门槛 |
| ENG-02 | 关键文档（架构/路线图/测试/AI审计） | docs 完整 | DONE | `docs/*` + `llms.txt` |
| ENG-03 | 一键审计打包脚本 | 生成 `review_bundle.zip` | DONE | `scripts/build_review_bundle.sh` |
| ENG-04 | 版本打点（tag/release） | `v0.1.0` tag + release notes | TODO | 等你确认“冻结”后执行 |
| ENG-05 | 可观测性（P0） | 最低限度 metrics/log | TODO | tick 延迟、队列长度、写入失败 |

## 2. 下一批推荐任务（短期 1-2 天）

| 优先级 | 任务 | 原因 |
|---:|---|---|
| P0 | SP-06 解说层模板化（基于事件类型/阶段） | 直接提升“观战爽感”，风险低（只增字段/文字） |
| P1 | ENG-04 打 tag：v0.1.0 | 形成“第一版”冻结点，便于回归与协作 |
| P1 | ENG-05 最小可观测性 | 长期运行必需，提前布局省后期返工 |

## 3. 更新记录（简版）

- 2026-04-18：增加 world stage/summary，并接入 replay；增加 “近期/风险/建议” beats；近期摘要去重；补齐对应集成测试。

