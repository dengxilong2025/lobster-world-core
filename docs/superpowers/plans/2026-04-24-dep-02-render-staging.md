# DEP-02：Render 免费 Staging（MVP）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 Render（Hobby/Free）部署 lobster-world-core staging，并把访问方式写入文档与 WBS。

**Architecture:** Render Web Service（Docker）。复用仓库 `Dockerfile`，监听 `PORT`；health check `/healthz`。

**Tech Stack:** Render Web UI + GitHub repo。

---

## Task 1: 文档与 WBS（先占坑）

**Files:**
- Create: `docs/ops/staging_render.md`
- Modify: `docs/wbs_v0_2.md`

- [ ] **Step 1: 新增部署说明文档骨架**

`docs/ops/staging_render.md` 至少包含：
- Render 服务名、仓库、分支
- Runtime: Docker
- Health check: `/healthz`
- 关键验证命令（curl）

- [ ] **Step 2: WBS 把 DEP-02 标记为 IN_PROGRESS**

- [ ] **Step 3: Commit**
```bash
git add docs/ops/staging_render.md docs/wbs_v0_2.md
git commit -m "docs(ops): add render staging guide (WIP)"
git push origin main
```

---

## Task 2: Render 创建服务（需要用户登录/授权）

- [ ] **Step 1: 登录 Render**
- [ ] **Step 2: New → Web Service**
- [ ] **Step 3: Connect GitHub 并选择仓库**
  - repo: `dengxilong2025/lobster-world-core`
  - branch: `main`
- [ ] **Step 4: 配置**
  - Environment: Hobby/Free
  - Runtime: Docker
  - Port: 使用 Render 默认 `PORT`（无需手填，确保 app 读 `PORT`）
  - Health check path: `/healthz`
- [ ] **Step 5: Deploy 并等待 ready**

---

## Task 3: 验收 smoke

- [ ] **Step 1: 记录 staging URL**
- [ ] **Step 2: curl 验收**
```bash
curl -sS -I "<STAGING_URL>/healthz" | head -n 1
curl -sS -I "<STAGING_URL>/ui" | head -n 1
curl -sS -I "<STAGING_URL>/assets/production/manifest.json" | head -n 1
```

---

## Task 4: 收口文档与 WBS（DONE）

- [ ] **Step 1: docs/ops/staging_render.md 填完整**
- [ ] **Step 2: docs/wbs_v0_2.md：DEP-02 → DONE，并写入 staging URL**
- [ ] **Step 3: Commit & push**
```bash
git add docs/ops/staging_render.md docs/wbs_v0_2.md
git commit -m "docs(wbs): mark DEP-02 done (render staging url)"
git push origin main
```

