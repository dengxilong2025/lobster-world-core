# 压测脚本与基准（auth / intents / sse）Design

**目标**：提供一套“零依赖、最小侵入、可复制”的本地压测脚本，用于在不同机器/不同配置下快速得到基准数据（吞吐、错误率、延迟概况），并在不影响 CI 稳定性的前提下辅助上线准备。

**范围**：
- 新增 `scripts/` 下的 bash 压测脚本（基于 `curl` + `xargs -P` 并发）
- 新增一份 `docs/ops/load_testing.md` 指南（如何启动 server、如何跑脚本、如何解读输出）

**非目标**：
- 不引入 k6/vegeta/hey 等外部工具依赖
- 不把压测纳入 CI（避免机器差异导致的 flaky）
- 不做复杂的 percentiles 统计（先给到 P50/P95 的近似或采样即可）

---

## 1) 脚本清单（计划）

1) `scripts/loadtest_auth_challenge.sh`
   - 目标：`POST /api/v0/auth/challenge`
   - 输出：QPS、状态码分布（200/429/400）、平均耗时（采样）

2) `scripts/loadtest_intents.sh`
   - 目标：`POST /api/v0/intents`
   - 可配置：`WORLD_ID`、`CONCURRENCY`、`REQUESTS`、`GOAL_PREFIX`
   - 输出：QPS、状态码分布（200/503 BUSY/400）、平均耗时（采样）

3) `scripts/loadtest_sse.sh`
   - 目标：`GET /api/v0/events/stream?world_id=...`
   - 可配置：连接数、持续时长
   - 输出：成功建立连接数、期间收到的 `data:` 行数（事件吞吐的粗略 proxy）

4) `scripts/loadtest_replay_export.sh`（可选）
   - 目标：`GET /api/v0/replay/export`
   - 输出：导出耗时与响应大小（bytes）

统一约定：
- 服务地址通过 `BASE_URL` 传入（默认 `http://localhost:8080`）
- 所有脚本在失败时返回非 0（便于在手动流水线中使用）

---

## 2) 输出格式（稳定可读）

每个脚本输出：
- 配置回显（BASE_URL / CONCURRENCY / REQUESTS / WORLD_ID）
- 统计摘要：
  - 总请求数、成功数、失败数
  - 状态码分布（按 code 聚合）
  - 总耗时、近似 QPS
- 如检测到 `503 BUSY` 占比过高，打印建议：提高 MaxIntentQueue / 降低并发 / 观察 debug/metrics

---

## 3) 与现有 debug/metrics 的协同

压测前后可手动查看：
- `GET /api/v0/debug/metrics`
验证：
- `requests_total` 增长
- `responses_by_status` 与脚本统计一致
- `busy_total` 是否异常上升

---

## 4) 风险与控制

- shell 统计不追求极致精度，目标是“快速对比不同配置/不同提交的趋势”
- 不做对外承诺的 SLO/SLA；仅作为内部基准与回归参考

