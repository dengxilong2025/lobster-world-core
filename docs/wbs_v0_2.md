# v0.2 开发 WBS / 进度看板（滚动更新）

> 用途：把“v0.2（像游戏的雏形：最薄前端壳 + 可被智能体测试）”拆解成可跟踪任务。  
> 维护方式：每次合并到 `main` 后更新本文件（建议按 commit 粒度更新）。  
> 状态枚举：`DONE` / `IN_PROGRESS` / `TODO` / `BLOCKED`

相关文档：
- 版本路线图：`docs/version_plan.md`
- v0.2 Web 壳计划：`docs/plans/2026-04-19_v0_2_web_shell_plan.md`
- /ui 使用手册：`docs/ui/v0_2_web_shell.md`

---

## 0) 总览

- 当前里程碑：**v0.2-M2：智能体体验测试通道（工程化）**
- 当前总体完成度（估算）：**80%**
- 当前状态（摘要）：M1/M2 已形成“可玩闭环 + 可批测 + 可留档对比”的最小交付；benchmarks v2 已升级到 v2.4（含 agent_batch 与 Verdict 回归来源清单）。剩余主要工作集中在：部署固定地址（DEP-02）与后续 v0.3/v0.4 内容线。

---

## 1) v0.2-M1：Web 雏形（最低可玩）

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| UI-01 | `/ui` 单页路由 | `GET /ui` 返回 HTML，包含稳定 DOM id | DONE | `internal/gateway/routes_ui.go` |
| UI-02 | `/ui` 提交意图 | fetch `/api/v0/intents` 可用；错误可见 | DONE | `internal/gateway/ui_page.go` |
| UI-03 | `/ui` SSE 事件流 | EventSource `/api/v0/events/stream?world_id=...` 可见 | DONE | 同上 |
| UI-04 | `/ui` 观战摘要 | 渲染 `/api/v0/spectator/home` 的 stage/summary | DONE | 同上 |
| UI-05 | `/ui` 回放入口 | 从 SSE 解析 event_id 生成 replay/highlight 链接 | DONE | 同上 |
| UI-06 | `/ui` 可脚本化 | 支持 `?world_id=...&goal=...&autoconnect=1` | DONE | 便于智能体批测 |
| UI-07 | 冒烟集成测试 | `TestUI_ServesHTML` 覆盖关键 DOM 与 endpoints | DONE | `tests/integration/ui_smoke_test.go` |
| UI-08 | 使用手册 | 人类最短路径 + 智能体 HTTP 测试路径 + 直达链接 | DONE | `docs/ui/v0_2_web_shell.md` |
| UI-09 | replay/export 入口 | /ui 提供 export 入口（按钮或链接） | DONE | `btn_export` |

---

## 2) v0.2-M2：智能体体验测试通道

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| AT-01 | 统一“智能体测试脚本” | 提供可重复运行脚本（或 Make target） | DONE | `scripts/agent_test_v0_2_m2.sh` |
| AT-02 | 批量 world 管理 | 支持批量创建 world_id / 回收策略（文档即可） | DONE | 见 /ui 使用手册“批量 world_id 管理策略” |
| AT-03 | 结果采集 | 每轮输出 replay/export（NDJSON）与失败原因 | DONE | 脚本输出 summary.json + ndjson |

---

## 3) v0.2 部署与工程化

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| DEP-01 | 一键启动 | docker compose 或单命令启动说明 | DONE | 已提供 Dockerfile + docker-compose.yml，并在 /ui 手册补充用法 |
| DEP-02 | 固定可访问地址（可选） | staging URL 或可转发方式 | IN_PROGRESS | Render（Hobby/Free） |

---

## 4) QA/证据链：Benchmarks & Diff（给回归/审计用）

> 目标：每次迭代都能留下可对比的证据：压测（QPS/latency）+ 批测（agent_batch）+ 解释性指标（tick/queue）+ 一眼可读的回归来源（Verdict breakdown）。

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| QA-01 | Benchmarks v2 归档 | 生成 `docs/ops/benchmarks/*.json/*.md/*.diff.md` | DONE | `scripts/benchmarks_v2.sh` |
| QA-02 | diff 解释性指标 | diff 含 `tick_overrun_total_sum` 与 `bench_world_pending_queue_len` | DONE | v2.2 |
| QA-03 | 纳入 agent_batch | benchmarks.json 含 `tests.agent_batch`；diff 含 `## agent_batch` | DONE | v2.3 |
| QA-04 | Verdict breakdown | Verdict 输出回归条目清单与计数（含 agent_batch） | DONE | v2.4 |

---

## 5) 仓库卫生（Repo hygiene）

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| RH-01 | 忽略本地产物 | `.gitignore` 忽略 `out/` / 本地 zip 等 | DONE | 避免污染 git status |
| RH-02 | 大文件策略（短期） | 规则明确；不强制 LFS | DONE | 后续若持续增长再迁移 |

---

## 6) 更新记录（简版）

- 2026-04-19：落地 `/ui` Web 壳（提交意图/SSE/摘要/回放），增加 query params + autoconnect，并补齐使用手册。
- 2026-04-23~24：benchmarks v2.2：diff 增加 tick/queue 摘要；完善样例/单测门禁并两次冒烟留档。
- 2026-04-24：v0.2-M2 批测脚本工程化（`scripts/agent_test_v0_2_m2.sh`），并将批测纳入 benchmarks（v2.3 agent_batch）。
- 2026-04-24：benchmarks v2.4：Verdict 输出回归条目清单与计数（明确回归来源）。
