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

- 当前里程碑：**v0.2-M1：Web 雏形（最低可玩）**
- 当前总体完成度（估算）：**55%**
- 当前状态（摘要）：M1 已形成可玩闭环（/ui：提交意图 / SSE / spectator.home / replay/highlight / replay/export），并补齐使用手册；当前进入 M2：把“智能体批测通道”工程化（脚本/产物/失败原因）。

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
| AT-01 | 统一“智能体测试脚本” | 提供可重复运行脚本（或 Make target） | IN_PROGRESS | `scripts/agent_test_v0_2_m2.sh` |
| AT-02 | 批量 world 管理 | 支持批量创建 world_id / 回收策略（文档即可） | TODO | 避免污染 |
| AT-03 | 结果采集 | 每轮输出 replay/export（NDJSON）与失败原因 | DONE | 脚本输出 summary.json + ndjson |

---

## 3) v0.2 部署与工程化

| ID | 任务 | 产出/验收标准 | 状态 | 备注 |
|---|---|---|---|---|
| DEP-01 | 一键启动 | docker compose 或单命令启动说明 | DONE | 已提供 Dockerfile + docker-compose.yml，并在 /ui 手册补充用法 |
| DEP-02 | 固定可访问地址（可选） | staging URL 或可转发方式 | TODO | 依赖你的部署环境 |

---

## 4) 更新记录（简版）

- 2026-04-19：落地 `/ui` Web 壳（提交意图/SSE/摘要/回放），增加 query params + autoconnect，并补齐使用手册。
