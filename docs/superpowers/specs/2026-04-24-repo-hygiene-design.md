# Repo Hygiene Design（2026-04-24）

## Goal

让仓库在多人/多机协作与持续迭代下保持“可持续、可 push、可复现”：

1) **避免本地产物污染 git 状态**（例如 `out/`、本地打包 zip、review bundle 等）
2) **减少 GitHub push 噪音**（例如大文件告警/误提交）
3) **默认工作流更顺滑**：`git status` 更干净，CI/审计材料通过“约定目录 + ignore”来管理

## Decisions

### D1：把“运行/产物目录”统一放到 `out/` 并默认忽略

在 `.gitignore` 增加：
- `out/`
- `review_bundle.zip`
- `lobster-world-core-*.zip`
- `lobster-world-core-project-*.zip`

### D2：大文件（>50MB）策略（短期）

短期不改历史、不强制 Git LFS（避免打断现有工作流），但：
- 通过 `.gitignore` 避免把临时 zip 重复提交
- 对 `assets/intake/**` 下的供应商交付 zip，保留现状（它们是源素材），后续若持续增长再做 LFS/外部存储迁移

## Non-goals

- 不在本次迭代中引入 Git LFS 并迁移历史（需要额外工具与历史重写）

