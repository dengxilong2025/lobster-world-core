# A1：staging smoke 脚本（Render）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `scripts/smoke_staging.sh`，一条命令快速验收 staging（/healthz、/→/ui、/ui、manifest、最小 intents→home→export 闭环），失败返回非 0。

**Architecture:** 纯 bash + curl；默认 `BASE_URL=https://lobster-world-core.onrender.com`，可通过环境变量覆盖；fail-fast；输出 OK/FAIL 行；world_id 使用 `smoke_<ts>` 保证幂等。

**Tech Stack:** bash、curl、（可选 python3/jq 用于 urlencode/JSON 检查，若不存在则降级）。

---

## 0) Files（锁定）

**Create:**
- `scripts/smoke_staging.sh`

**Modify (可选):**
- `README.md`（加一行 “staging smoke” 用法）

---

## Task 1: TDD — 先写一个最小的自测（RED）

> 说明：bash 脚本的“单测”不追求完整，只做一个轻量 guard：确保脚本能打印 usage/help，且包含关键检查项文案，防止未来误删。

- [ ] **Step 1: 新增脚本自测（bash）**

Create: `scripts/smoke_staging_test.sh`
```bash
#!/usr/bin/env bash
set -euo pipefail

out="$(bash scripts/smoke_staging.sh --help || true)"
echo "$out" | grep -q "BASE_URL"
echo "$out" | grep -q "/healthz"
echo "$out" | grep -q "/api/v0/intents"
echo "OK"
```

- [ ] **Step 2: 运行确认失败（RED）**
```bash
bash scripts/smoke_staging_test.sh
```
Expected: FAIL（因为 smoke_staging.sh 还不存在）

- [ ] **Step 3: Commit（仅测试）**
```bash
git add scripts/smoke_staging_test.sh
git commit -m "test(scripts): gate staging smoke script help output"
```

---

## Task 2: 实现 scripts/smoke_staging.sh（GREEN）

- [ ] **Step 1: 创建脚本**

Create: `scripts/smoke_staging.sh`

要求：
- 支持 `--help`
- 默认 `BASE_URL=https://lobster-world-core.onrender.com`
- 检查：
  - `GET /healthz` => 200
  - `GET /` => 302 且 Location=/ui
  - `GET /ui` => 200
  - `GET /assets/production/manifest.json` => 200
  - `POST /api/v0/intents` => 200 且响应体含 `"ok":true`
  - `GET /api/v0/spectator/home?world_id=...` => 200
  - `GET /api/v0/replay/export?...&limit=200` => 200
- 任一步失败：打印 FAIL 行并 exit 1；成功：最后打印 ALL OK 并 exit 0

- [ ] **Step 2: 运行脚本自测转绿**
```bash
bash scripts/smoke_staging_test.sh
```
Expected: PASS

- [ ] **Step 3: 本地 dry-run（不要求服务在线）**
```bash
BASE_URL=http://localhost:8080 bash scripts/smoke_staging.sh || true
```
Expected: 若本地没起服务会 FAIL（这是正常的），但输出应清晰指明失败点。

- [ ] **Step 4: Commit（脚本实现）**
```bash
git add scripts/smoke_staging.sh
git commit -m "feat(scripts): add staging smoke script"
```

---

## Task 3: 文档与推送

- [ ] **Step 1: 可选更新 README**
在 README 增加：
```bash
bash scripts/smoke_staging.sh
```

- [ ] **Step 2: push**
```bash
git push origin main
```

