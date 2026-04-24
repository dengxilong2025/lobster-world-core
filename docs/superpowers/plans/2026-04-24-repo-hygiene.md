# Repo Hygiene Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 清理并固化仓库 hygiene：忽略本地产物目录与临时 zip，减少 git 状态污染与误提交。

**Architecture:** 通过 `.gitignore` 的规则实现，不引入新依赖；如需更强的大文件治理，后续再单独引入 Git LFS/外部存储方案。

**Tech Stack:** git + .gitignore。

---

## Task 1: 落地 ignore 规则

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: 增加 ignore**
```gitignore
out/
review_bundle.zip
lobster-world-core-*.zip
lobster-world-core-project-*.zip
```

- [ ] **Step 2: 验证**
```bash
git status --porcelain
```
Expected: `out/` 与本地 zip 不再显示为未跟踪文件。

- [ ] **Step 3: Commit & push**
```bash
git add .gitignore
git commit -m "chore: ignore local output artifacts"
git push origin main
```

