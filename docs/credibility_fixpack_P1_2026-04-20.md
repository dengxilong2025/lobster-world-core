# 可信度整改包（P1）— 2026-04-20

> 目的：让 lobster-world-core 的“主链路”在第一次体验时更稳、更可信、更接近可部署状态。  
> 原则：**不堆更多玩法**，优先补齐“连通性、并发安全、部署一致性、安全语义”的底座。

---

## P1-A 修复 /ui SSE 主路径 + 同步文档

### 背景
- `/ui` Web Shell 里 SSE 使用了 `/api/v0/events?world_id=...`
- 后端真正的 SSE 路由是 `/api/v0/events/stream?world_id=...`
- 这会导致 UI 在真实浏览器里“连接事件流”不稳定或直接不通，严重影响第一印象。

### 交付
- 修正 `internal/gateway/ui_page.go` 的 SSE 地址到 `/api/v0/events/stream`
- 更新相关文档：
  - `docs/ui/v0_2_web_shell.md`
  - `docs/wbs_v0_2.md`（UI-03 相关描述）
- 更新测试中对 SSE 路径的断言（避免回归）

### 验收
- 打开 `/ui` 点击“连接事件流”后状态显示“已连接”
- 提交意图后，事件区能滚动输出新增事件

---

## P1-B 增加真实 UI 主链路冒烟测试（SSE 连通性）

### 背景
目前 UI 测试偏“HTML 包含字符串”，容易漏掉真实连通性问题（例如 SSE 路径写错）。

### 交付
- 新增集成测试：启动 server → 发起 SSE 连接 → 触发事件（提交 intent）→ 断言 SSE 收到 `data:` 行（包含 world_id / event_id 等字段）

### 验收
- `go test ./...` 通过
- 测试在 SSE 路径错误时应失败（能作为门禁）

---

## P1-C auth.Service 并发安全（map 加锁）

### 背景
`auth.Service` 在 HTTP 并发下会被同时访问，但内部使用 `map` 保存 challenge/session 未加锁，存在 data race 风险。

### 交付
- 给 `internal/auth/service.go` 增加 `sync.RWMutex`
- 对 `CreateChallenge / Prove / GetSession` 中所有 `map` 读写加锁
- （可选）补充一个并发测试或在开发文档里建议 `go test -race` 用法

### 验收
- `go test ./...` 通过
- 在并发压测下不出现竞态（后续可纳入 CI）

---

## P1-D 收紧认证限流的 IP 识别边界（X-Forwarded-For）

### 背景
当前限流无条件信任 `X-Forwarded-For`，在服务可直连时，客户端可伪造该头绕过限流。

### 交付（最小但靠谱）
- 默认使用 `RemoteAddr` 作为 client IP
- 仅当请求来自“受控反代”时才信任 `X-Forwarded-For`
  - MVP 策略：只信任 `RemoteAddr` 为 loopback（127.0.0.1 / ::1）的请求携带的 XFF
- 在文档写清楚：如需部署在反代后，应如何配置与预期行为

### 验收
- 直连请求伪造 XFF 不再绕过限流
- 反代（localhost）转发场景仍正常

---

## P1-E Docker 资源打包一致性（assets/production）

### 背景
容器镜像若只包含 server 二进制，不包含 `assets/production`，则 `/ui/assets` 与静态资源在容器里不可用，容易在演示/部署时翻车。

### 交付
- 更新 `Dockerfile`：把 `assets/production` 一起 COPY 到运行时镜像（例如 `/assets/production`）
- 保持现有 `assetProductionDir()` 可自动找到 `/assets/production`
- 增补一段容器态验证步骤（README 或 docs）

### 验收
- `docker build && docker run` 后：
  - `GET /ui/assets` 返回 200
  - `GET /assets/production/manifest.json` 返回 200

---

## P1-F adoption 最小防重放（nonce 去重 + client_ts 时间窗）

### 背景
领养/解绑签名消息里已有 `nonce` 与 `client_ts`，但服务端未做去重与时间窗校验，防重放语义未闭环。

### 交付（最小版本）
- 校验 `client_ts` 在允许窗口内（建议 ±5 分钟）
- 对 `(humanID, lobsterID, nonce)` 做 TTL 缓存去重（例如 10 分钟）
- 重复 nonce 或时间窗外请求直接拒绝
- 增加测试覆盖：同 nonce 第二次失败；过期 client_ts 失败

### 验收
- 重放请求被拒绝
- 正常签名流程不受影响

---

## 推进顺序（执行顺序）
1) P1-A（SSE 修复）  
2) P1-B（SSE 冒烟测试）  
3) P1-C（auth 并发安全）  
4) P1-D（限流 IP 边界）  
5) P1-E（Docker 资源一致性）  
6) P1-F（adoption 防重放）  
7) 同步更新 WBS/文档（每做完一项就同步，避免漂移）

