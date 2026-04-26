# A2.1：强保证 debug/build 输出 git_sha Design（2026-04-26）

## 背景

当前 `GET /api/v0/debug/build` 已上线，但在 Render 的构建链路下经常无法从 `runtime/debug.ReadBuildInfo()` 读取到 `vcs.revision`，导致 `git_sha` 缺失，影响“确认线上跑的是哪次 commit”。

## Goal

让 `GET /api/v0/debug/build` **稳定输出**：
- `git_sha`：**短 SHA（7 位）**，任何环境都尽量能给出（强保证：至少返回非空字符串）

## Non-goals

- 不引入复杂的版本服务/Release 系统
- 不改现有业务 API 行为
- 不在本轮做鉴权（保持与其它 debug 端点一致）

---

## 方案（推荐）

### 1) 代码侧：增加可注入的 buildGitSHA 兜底

在 `internal/gateway/buildinfo.go` 增加：
- `var buildGitSHA string`（默认空，由构建时注入）

在 `buildInfoSnapshot()` 中：
1. 若 `debug.ReadBuildInfo()` 有 `vcs.revision` → 取其前 7 位写入 `git_sha`
2. 否则若运行环境提供 `RENDER_GIT_COMMIT`（Render 默认环境变量）→ 取其前 7 位写入 `git_sha`
3. 否则若 `buildGitSHA` 非空 → 写入 `git_sha=buildGitSHA`
4. 否则 → 写入 `git_sha="unknown"`（保证字段不空）

### 2) 构建侧：Dockerfile 使用 ldflags 注入短 SHA

修改 `Dockerfile` builder 阶段的 go build 行：
- 在构建容器内尝试运行 `git rev-parse --short HEAD`
- 若仓库没有 `.git`（或 git 不可用），则降级为 `unknown`
- 通过 `-ldflags "-X lobster-world-core/internal/gateway.buildGitSHA=${GIT_SHA} -s -w"` 注入

> 说明：优先保证“永远不因为拿不到 git sha 而构建失败”，所以取不到时要降级。

---

## 验收标准

在 staging：
- `curl -sS https://lobster-world-core.onrender.com/api/v0/debug/build`
  - `build.git_sha` 必须存在且非空
  - 长度应为 7（除非降级为 `"unknown"`，则为 7+ 字符串，但同样非空）

本地：
- `go test ./...` 通过
- integration test 覆盖：当 `buildGitSHA` 被设置时，`git_sha` 不为空
