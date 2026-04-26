# A2：/api/v0/debug/build（版本可见性）Design（2026-04-26）

## 背景 / 痛点

staging/线上排障时，经常需要先回答两个问题：
1) **现在线上跑的是哪次 commit？**（是否已部署到预期版本）
2) **服务是否刚重启 / 是否冷启动？**

当前已有：
- `GET /api/v0/debug/config`
- `GET /api/v0/debug/metrics`

但缺少“版本可见性（build info）”端点。

---

## Goal

新增一个最小侵入的 debug 端点：
- `GET /api/v0/debug/build`

用于返回：
- `git_sha` / `vcs_time` / `vcs_modified`（尽可能从 Go build info 中提取）
- `go_version`、`module`（build metadata）
- `start_time`、`uptime_sec`（进程启动与运行时长）

---

## Non-goals

- 不引入 Prometheus/OpenTelemetry
- 不做鉴权（保持和其它 debug 端点一致的“最小可用”策略）
- 不强制改 Dockerfile 注入 ldflags（本轮先依赖 Go 的 buildvcs 信息；若后续发现缺失，再升级）

---

## API 设计

### Path

`GET /api/v0/debug/build`

### Response（JSON）

```json
{
  "ok": true,
  "build": {
    "git_sha": "b46748e",
    "vcs_time": "2026-04-26T04:23:54Z",
    "vcs_modified": false,
    "go_version": "go1.22.6",
    "module": "lobster-world-core",
    "module_version": "",
    "start_time": "2026-04-26T04:24:05Z",
    "uptime_sec": 123
  }
}
```

说明：
- `git_sha/vcs_time/vcs_modified`：从 `runtime/debug.ReadBuildInfo()` 的 settings 中读取 `vcs.revision/vcs.time/vcs.modified`，如缺失则留空/省略字段（实现时统一策略）。
- `uptime_sec`：用 `time.Since(start).Seconds()` 向下取整。

---

## 实现要点

1) 在 gateway 包里增加一个 `startTime`（package-level var），在 init 或 `NewHandler` 时初始化一次。
2) 新增 debug route handler（放在 `routes_debug.go`）：
   - 读取 `debug.ReadBuildInfo()`（若 ok=false，则仅返回 start_time/uptime/go_version 兜底）
   - 组装 JSON 并 `writeJSON`
3) 新增一个轻量 integration test：
   - httptest 启动 app
   - `GET /api/v0/debug/build` 断言 200、`ok=true`
   - 至少断言 `start_time` 字段存在、`uptime_sec>=0`
   - `git_sha` 若存在则应为非空字符串（允许缺省）

---

## 验收标准

- staging 调用 `GET https://lobster-world-core.onrender.com/api/v0/debug/build` 返回 200，且能看到 start_time/uptime_sec。
- 若 buildvcs 信息可用：能看到 `git_sha` 与 `vcs_time`（便于确认部署版本）。

