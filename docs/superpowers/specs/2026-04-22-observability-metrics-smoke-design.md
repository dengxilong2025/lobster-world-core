# 可观测性/指标（metrics）+ 端到端 smoke 门禁 Design

**目标**：在不偏离当前“MVP 事件溯源 + 投影 + 仿真”的初始设计前提下，补齐两类工程化护栏：

1) **端到端 smoke 门禁（integration test）**：覆盖“外交/贸易 intents → 事件出现 → export 可回放”，并在关键阶段校验 spectator/home 的建议仍可引导玩家触发新事件类型。
2) **轻量 metrics**：提供一个最小侵入、零依赖的调试指标端点，便于观察请求量/错误量/BUSY 等系统健康信号。

---

## 1) 端到端 smoke 门禁（优先级更高）

### 1.1 设计原则
- **用黑盒 API 驱动**（httptest + HTTP），避免直接改内部状态
- **决定论**：固定 seed + 快 tick，断言“出现某类事件”，不强绑定 actor ID
- **覆盖关键链路**：
  - `POST /api/v0/intents`
  - `GET /api/v0/spectator/home`
  - `GET /api/v0/replay/export`

### 1.2 验收点（门禁断言）
1) 连续提交若干 `背叛` intent 抬高 conflict，`/spectator/home` 的 `world.summary` 必须出现：
   - 至少一条 `建议：` 且包含外交关键词（停战/谈判/条约/结盟）
   - 且包含预期事件字符串：`alliance_formed` 或 `treaty_signed`
2) 提交 `结盟` intent 后，export 中必须出现 `alliance_formed`
3) 提交 `条约/谈判/停战` intent 后，export 中必须出现 `treaty_signed`
4) 提交 `贸易/商路/集市/交换` intent 后，export 中必须出现 `trade_agreement`
5) export 仍保持确定性与可验证性（已有门禁覆盖排序/版本字段）

---

## 2) 轻量 metrics（debug JSON）

### 2.1 暴露方式
新增：
- `GET /api/v0/debug/metrics`

返回 JSON（示例）：
```json
{
  "ok": true,
  "metrics": {
    "requests_total": 123,
    "responses_by_status": {"200": 120, "400": 2, "503": 1},
    "busy_total": 1
  }
}
```

### 2.2 埋点策略（最小侵入）
- 在 `NewHandler` 返回的 mux 外层包一层 `metricsMiddleware`：
  - 每个请求：`requests_total++`
  - 记录 HTTP status：`responses_by_status[code]++`
- 在 `writeError` 里，当 `code=="BUSY"` 时额外 `busy_total++`（用于区分一般 503）

> 说明：这仍不引入第三方依赖，不改变既有 API，只新增 debug 端点与 middleware。

---

## 3) 风险控制
- metrics 仅用于 debug：不承诺 Prometheus 格式、不做高基数 label（只做 path 级别或只做总量）
- 测试断言使用“出现事件类型”而非全文匹配，减少不必要的脆弱性

