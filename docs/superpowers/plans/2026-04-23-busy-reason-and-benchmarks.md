# BUSY reason 拆分 + benchmarks 归档自动化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `503 BUSY` 拆分为可解释的背压来源并在 debug/metrics 输出 `busy_by_reason`；新增一键 benchmarks 归档脚本生成 `docs/ops/benchmarks/<date>_<sha>.md`。

**Architecture:** sim 侧引入 `BusyError{Reason}` 并保持 `errors.Is(err, ErrBusy)` 兼容；gateway 在 `/api/v0/intents` 通过 `errors.As` 解析 reason 并计数到 metrics。benchmarks 脚本复用现有 loadtest_*，并抓取 debug/config + debug/metrics 写入 markdown。

**Tech Stack:** Go、bash、curl、awk。

---

## 0) Files

**Create:**
- `internal/sim/busy_error.go`
- `scripts/benchmarks_run.sh`
- `docs/ops/benchmarks/.gitkeep`
- `tests/integration/busy_reason_metrics_test.go`

**Modify:**
- `internal/sim/world.go`
- `internal/sim/engine.go`
- `internal/gateway/metrics.go`
- `internal/gateway/routes_intents.go`
- `internal/gateway/routes_debug.go`（如需）

---

## Task 1: BUSY reason（TDD：先红）

**Files:**
- Create: `tests/integration/busy_reason_metrics_test.go`

- [ ] **Step 1: 写 failing integration test（pending_queue_full）**

思路：用 `TickInterval=5s` + `MaxIntentQueue=1`，连续提交 2 个 intents，第二个返回 503 BUSY。随后 GET `/api/v0/debug/metrics`，断言 `busy_by_reason.pending_queue_full >= 1`。

（测试中解析 metrics：复用现有 `getMetricsMap`/`metricInt64` helper。）

- [ ] **Step 2: 写 failing integration test（intent_ch_full）**

思路：创建一个 world 并快速并发提交 intents（高并发 + 小 `intentCh` 容量不易直接配置）。
为保持最小侵入：在 sim 暴露一个 **仅测试用** 的 options（不推荐）不合适；因此本测试采用“白盒地填满 intentCh”也不合适。

v1 交付策略：先只覆盖 `pending_queue_full` + `accept_timeout`（两者更稳定可复现），`intent_ch_full` 放到 v2.1。

> 本步骤如果确认要做 intent_ch_full，需要先在 sim.Options 加 `IntentChannelCap`（默认 256）以便测试可控。

- [ ] **Step 3: 写 failing integration test（accept_timeout）**

思路：复用 sim 的 blocking store（内部已有 `engine_timeout_test.go`），在 gateway 侧配置 `IntentAcceptTimeout=10ms`，让 SubmitIntent 超时。断言：
- HTTP 返回 500（或 503？需要在 gateway 映射：建议对 accept_timeout 仍视作 BUSY→503）
- metrics 的 `busy_by_reason.accept_timeout` 增长

- [ ] **Step 4: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run BusyReason -v
```

- [ ] **Step 5: Commit（仅测试）**
```bash
git add tests/integration/busy_reason_metrics_test.go
git commit -m "test(debug): add busy reason metrics gates"
```

---

## Task 2: BUSY reason（实现转绿）

**Files:**
- Create: `internal/sim/busy_error.go`
- Modify: `internal/sim/world.go`
- Modify: `internal/sim/engine.go`
- Modify: `internal/gateway/metrics.go`
- Modify: `internal/gateway/routes_intents.go`
- Modify: `internal/gateway/routes_debug.go`（如需）

- [ ] **Step 1: sim 新增 BusyError**

`internal/sim/busy_error.go`：
```go
package sim

type BusyReason string

const (
  BusyReasonIntentChFull     BusyReason = "intent_ch_full"
  BusyReasonPendingQueueFull BusyReason = "pending_queue_full"
  BusyReasonAcceptTimeout    BusyReason = "accept_timeout"
)

type BusyError struct{ Reason BusyReason }

func (e BusyError) Error() string { return "busy: " + string(e.Reason) }
func (e BusyError) Is(target error) bool { return target == ErrBusy }
```

- [ ] **Step 2: world/engine 返回带 reason 的 BusyError**

在 `world.submitIntent`：
- channel 满 → `return "", nil, BusyError{Reason: BusyReasonIntentChFull}`

在 `world.handleIntent` queue 满：
- `qi.Ack <- BusyError{Reason: BusyReasonPendingQueueFull}`

在 `engine.SubmitIntent` 超时：
- `return "", BusyError{Reason: BusyReasonAcceptTimeout}`

- [ ] **Step 3: gateway metrics 增加 busy_by_reason**

在 `internal/gateway/metrics.go`：
- 维护 `busyByReason` map（低基数）或固定原子字段（推荐固定字段）
  - `busyIntentChFullTotal`
  - `busyPendingQueueFullTotal`
  - `busyAcceptTimeoutTotal`
- Snapshot 增加：
```json
"busy_by_reason": {"intent_ch_full": X, "pending_queue_full": Y, "accept_timeout": Z}
```

- [ ] **Step 4: gateway 解析 BusyError 并计数**

在 `/api/v0/intents` handler 的 BUSY 分支：
- `mt.IncBusy()`
- `var be sim.BusyError; if errors.As(err, &be) { ... }`

同时决定 accept_timeout 的 HTTP 映射：
- 建议也映射为 503 BUSY（因为客户端角度就是“系统忙/没接收成功”）

- [ ] **Step 5: 运行测试转绿 + 全量回归**
```bash
go test ./tests/integration -run BusyReason -v
go test ./...
```

- [ ] **Step 6: Commit（实现）**
```bash
git add internal/sim/busy_error.go internal/sim/world.go internal/sim/engine.go internal/gateway/metrics.go internal/gateway/routes_intents.go
git commit -m "feat(debug): split busy reasons"
```

---

## Task 3: benchmarks 归档脚本

**Files:**
- Create: `docs/ops/benchmarks/.gitkeep`
- Create: `scripts/benchmarks_run.sh`

- [ ] **Step 1: 创建目录占位**
```bash
mkdir -p docs/ops/benchmarks
```

- [ ] **Step 2: 写脚本**

脚本行为：
- 读取 `BASE_URL`（默认 http://localhost:8080）
- 读取 git sha：`git rev-parse --short HEAD`
- 日期：`date +%F`
- 运行：
  - `loadtest_auth_challenge.sh`（小规模）
  - `loadtest_intents.sh`（中规模）
  - `loadtest_replay_export.sh`（小规模）
- 拉取：
  - `/api/v0/debug/config`
  - `/api/v0/debug/metrics`
- 输出 markdown 到：
  - `docs/ops/benchmarks/${date}_${sha}.md`

- [ ] **Step 3: 冒烟运行并确认生成文件**

- [ ] **Step 4: Commit**
```bash
git add scripts/benchmarks_run.sh docs/ops/benchmarks/.gitkeep docs/ops/benchmarks/*.md
git commit -m "chore(ops): add benchmarks archive script"
```

---

## Task 4: 交付覆盖包

- [ ] **Step 1: format-patch**
```bash
git format-patch -6 -o /workspace/patches_busy_reason_bench
cd /workspace && zip -qr patches_busy_reason_bench.zip patches_busy_reason_bench
```

