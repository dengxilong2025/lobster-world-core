# DEP-02：Render 免费 Staging（MVP）Design（2026-04-24）

## Goal

提供一个**免费的、可对外访问的 staging URL**，用于：
- 给外部/多智能体复现 v0.2-M1/M2 的最小闭环（/ui + /api/v0/*）
- 让 benchmarks v2（含 agent_batch）可以对同一 URL 反复跑，形成可对比证据链

用户已确认：
- 采用方案 B（云平台）→ **Render Hobby/Free**
- 可接受免费层的睡眠/冷启动

## Key Decisions

1) **部署方式：Docker Web Service**
   - 直接使用仓库现有 `Dockerfile`（Go 1.22 builder + distroless runtime）
   - Render 自动提供 `PORT` 环境变量，server 已支持 `PORT`（默认 8080）

2) **健康检查：`/healthz`**
   - Render health check path 配置为 `/healthz`

3) **最小配置：不引入 DB、不改 API**
   - 保持当前“内存态 EventStore”，staging 重启视为清空
   - 不在本阶段引入任何持久化依赖，避免增加成本与复杂度

## Success Criteria

部署完成后满足：
- `GET <staging>/healthz` 返回 200
- `GET <staging>/ui` 可打开
- `POST <staging>/api/v0/intents` 可用
- `GET <staging>/assets/production/manifest.json` 返回 200（用于 /ui/assets）

## Docs / WBS Updates

部署完成后更新：
- `docs/wbs_v0_2.md`：
  - DEP-02: TODO → DONE，并写入 staging URL
- 新增 `docs/ops/staging_render.md`：
  - Render 创建服务的关键配置项（repo/branch/runtime/health check/env）
  - 如何本地/远端跑 smoke（curl 命令）

