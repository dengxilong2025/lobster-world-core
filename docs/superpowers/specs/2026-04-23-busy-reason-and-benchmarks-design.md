# BUSY reason 拆分 + benchmarks 归档自动化 Design

**目标**：在已有 debug/metrics（v1 + v2-queue）基础上，进一步提升“解释性”和“可回归性”：

1) **BUSY reason 拆分**：将 `503 BUSY` 归因到具体背压来源（入口 channel 满 / pending queue 满 / accept timeout），并在 debug/metrics 增加 `busy_by_reason`。
2) **benchmarks 归档自动化**：提供一个“一键跑压测并归档”的脚本，把每次结果落盘到 `docs/ops/benchmarks/<date>_<gitsha>.md`，包含环境信息、loadtest 输出摘要、debug/metrics 快照。

**非目标**：
- 不引入 Prometheus、k6、vegeta
- 不将 benchmarks 进入 CI（避免 flaky）

---

## 1) BUSY reason 拆分

### 1.1 现状

目前 gateway 仅能判断 `errors.Is(err, sim.ErrBusy)`，并计数到 `busy_total`，无法区分：
- `world.intentCh` 满导致的立即拒绝（入口突发）
- `pending queue` 满导致的拒绝（世界处理不过来）
- `SubmitIntent` 等待 accept 超时（持久化阻塞或世界 loop 卡死）

### 1.2 设计

新增错误类型（sim 层）：
- `type BusyError struct { Reason string }`
- `func (e BusyError) Error() string`
- `func (e BusyError) Is(target error) bool { return target == ErrBusy }`

Reason 枚举（字符串常量）：
- `intent_ch_full`
- `pending_queue_full`
- `accept_timeout`

sim 侧产出：
- `world.submitIntent` 在 `intentCh` 写入失败时返回 `BusyError{Reason:"intent_ch_full"}`
- `world.handleIntent` 在 `len(queue)>=maxQueue` 时向 ack 写 `BusyError{Reason:"pending_queue_full"}`
- `engine.SubmitIntent` 在等待 ack 超时返回 `BusyError{Reason:"accept_timeout"}`

gateway 侧消费：
- 保持 `errors.Is(err, sim.ErrBusy)` 逻辑不变（兼容旧代码）
- 额外 `errors.As(err, *sim.BusyError)` 获取 reason
- metrics 新增：
  - `busy_by_reason`：`{"intent_ch_full": X, "pending_queue_full": Y, "accept_timeout": Z}`

### 1.3 测试（TDD）

新增/扩展测试：
- sim 单测：
  - 小 `intentCh` capacity（或强制塞满）触发 `intent_ch_full`
  - `MaxIntentQueue=1` 触发 `pending_queue_full`
  - 使用 blocking store + 小 `IntentAcceptTimeout` 触发 `accept_timeout`
- gateway/integration：
  - 通过配置与并发请求触发对应 BUSY
  - 断言 `/api/v0/debug/metrics` 中 `busy_by_reason` 对应 key 增长

---

## 2) benchmarks 归档自动化

### 2.1 设计

新增脚本：
- `scripts/benchmarks_run.sh`

行为：
1) 读取 `BASE_URL`（默认 localhost:8080）
2) 获取当前 git sha（`git rev-parse --short HEAD`）
3) 运行固定组合的 loadtest（轻量，不求精确）：
   - auth challenge（小规模）
   - intents（中规模）
   - replay/export（小规模）
4) 拉取 `/api/v0/debug/metrics` 与 `/api/v0/debug/config` 快照
5) 生成 Markdown 文件：
   - 路径：`docs/ops/benchmarks/YYYY-MM-DD_<sha>.md`
   - 内容：环境（OS/Go 版本/sha/BASE_URL）、每段脚本输出摘要、metrics/config JSON（截断到合理长度）

### 2.2 风险与控制
- 仅沉淀“趋势”数据，避免精确 percentiles
- 文件体积控制：对 JSON 做截断（head N chars）或只保留关键字段

