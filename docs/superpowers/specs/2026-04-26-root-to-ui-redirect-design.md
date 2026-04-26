# Root（/）重定向到 /ui Design（2026-04-26）

## 背景 / 问题

当前 staging（Render）根路径：
- `GET /` 返回 `404 page not found`
- `GET /ui` 返回 200（UI 正常）

用户访问域名根路径时容易误判“服务挂了”。

## Goal

让 `https://<host>/` 作为更友好的入口：
- 访问 `/` 自动跳转到 `/ui`
- 不影响现有 `/ui`、`/healthz`、`/api/v0/*` 行为

## 方案（用户已确认）

### 行为

新增路由：
- `GET /` → `302` 重定向到 `/ui`
- 保留 query string：
  - `/?world_id=w1&goal=...` → `/ui?world_id=w1&goal=...`

### 测试

新增/扩展集成测试，至少覆盖：
- `GET /` 返回 302
- `Location` 以 `/ui` 开头
- 带 query 时 `Location` 包含原 query

## Non-goals

- 不新增 landing page（本次只做重定向）
- 不引入 SEO/永久重定向策略（默认 302）

