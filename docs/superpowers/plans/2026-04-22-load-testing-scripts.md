# 压测脚本与基准（auth / intents / sse）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 增加一套零依赖（bash+curl）、可复制的本地压测脚本与使用指南，用于快速得到 auth/intents/sse/export 的吞吐/错误率/耗时摘要；不进入 CI。

**Architecture:** 每个脚本使用 `curl -w "%{http_code} %{time_total}\n"` 输出单行结果，再用 `awk` 聚合状态码分布、平均耗时与近似 QPS；SSE 使用 `timeout` 控制持续时长并统计 `data:` 行数。文档说明如何结合 `/api/v0/debug/metrics` 验证计数一致性。

**Tech Stack:** bash、curl、awk、timeout（coreutils）。

---

## 0) Files 结构与改动范围（先锁定）

**Create:**
- `scripts/loadtest_auth_challenge.sh`
- `scripts/loadtest_intents.sh`
- `scripts/loadtest_sse.sh`
- `scripts/loadtest_replay_export.sh`
- `docs/ops/load_testing.md`

**Modify:**
- `docs/roadmap.md`（阶段 4：压测脚本与基准标记完成）

---

## Task 1: 实现 loadtest_auth_challenge.sh

**Files:**
- Create: `scripts/loadtest_auth_challenge.sh`

- [ ] **Step 1: 写脚本**

脚本要求：
- 环境变量：
  - `BASE_URL`（默认 `http://localhost:8080`）
  - `CONCURRENCY`（默认 10）
  - `REQUESTS`（默认 200）
- 以固定 payload 请求：`{"wallet":"w_test"}`（若接口校验更严格则按现有最小合法结构）
- 每次请求输出：`<http_code> <time_total>`
- 汇总输出：总耗时、近似 QPS、状态码分布、平均耗时

- [ ] **Step 2: 本地冒烟**
```bash
./scripts/loadtest_auth_challenge.sh
```

- [ ] **Step 3: Commit**
```bash
git add scripts/loadtest_auth_challenge.sh
git commit -m "chore(loadtest): add auth challenge load script"
```

---

## Task 2: 实现 loadtest_intents.sh

**Files:**
- Create: `scripts/loadtest_intents.sh`

- [ ] **Step 1: 写脚本**

环境变量：
- `BASE_URL`（默认 `http://localhost:8080`）
- `WORLD_ID`（默认 `w_load`）
- `CONCURRENCY`（默认 10）
- `REQUESTS`（默认 200）
- `GOAL`（默认 `启动世界`）

统计：
- 状态码分布（重点观察 503）
- 若 503 比例较高，打印提示：检查 `/api/v0/debug/metrics` 的 `busy_total`

- [ ] **Step 2: Commit**
```bash
git add scripts/loadtest_intents.sh
git commit -m "chore(loadtest): add intents load script"
```

---

## Task 3: 实现 loadtest_sse.sh

**Files:**
- Create: `scripts/loadtest_sse.sh`

- [ ] **Step 1: 写脚本**

环境变量：
- `BASE_URL`（默认 `http://localhost:8080`）
- `WORLD_ID`（默认 `w_load`）
- `CONNECTIONS`（默认 10）
- `DURATION_SEC`（默认 10）

行为：
- 先建立 SSE 连接（并发 CONNECTIONS）
- 每个连接使用 `timeout ${DURATION_SEC}s curl -sN ...`
- 输出：成功连接数、每条连接收到的 `data:` 行数（事件数 proxy）

- [ ] **Step 2: Commit**
```bash
git add scripts/loadtest_sse.sh
git commit -m "chore(loadtest): add sse load script"
```

---

## Task 4: 实现 loadtest_replay_export.sh（可选但推荐）

**Files:**
- Create: `scripts/loadtest_replay_export.sh`

- [ ] **Step 1: 写脚本**

环境变量：
- `BASE_URL`（默认 `http://localhost:8080`）
- `WORLD_ID`（默认 `w_load`）
- `REQUESTS`（默认 50）
- `CONCURRENCY`（默认 5）
- `LIMIT`（默认 5000）

输出：
- 平均耗时
- 平均响应大小（bytes）

- [ ] **Step 2: Commit**
```bash
git add scripts/loadtest_replay_export.sh
git commit -m "chore(loadtest): add replay export load script"
```

---

## Task 5: 写 docs/ops/load_testing.md 指南

**Files:**
- Create: `docs/ops/load_testing.md`

- [ ] **Step 1: 写文档**

包含：
- 如何启动服务（本地）
- 如何跑四个脚本（含示例命令）
- 如何结合 `/api/v0/debug/metrics` 交叉验证
- 常见问题：429（auth 限流）、503（busy 背压）、SSE 连接断开

- [ ] **Step 2: Commit**
```bash
git add docs/ops/load_testing.md
git commit -m "docs(ops): add load testing guide"
```

---

## Task 6: 收尾（roadmap + 补丁包）

- [ ] **Step 1: 更新 roadmap**
将“压测脚本与基准（auth/intents/sse）”标记完成。

- [ ] **Step 2: go test（确保不破坏单测）**
```bash
go test ./...
```

- [ ] **Step 3: 打补丁包**
```bash
git format-patch -6 -o /workspace/patches_load_testing
cd /workspace && zip -qr patches_load_testing.zip patches_load_testing
```

