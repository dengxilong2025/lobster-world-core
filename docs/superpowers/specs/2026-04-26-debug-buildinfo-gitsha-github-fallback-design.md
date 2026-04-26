# A2.2：GitHub API 兜底 debug/build 的 git_sha（Render）Design（2026-04-26）

## 背景 / 问题

当前 `/api/v0/debug/build` 已 **强保证** `git_sha` 非空，但在 Render（Docker runtime）上仍可能出现：
- `git_sha="unknown"`

原因：
- buildvcs 的 `vcs.revision` 缺失（`runtime/debug.ReadBuildInfo` 取不到）
- Docker build 阶段缺少 `.git`，无法从 `git rev-parse` 得到 commit
- 运行时预期的 `RENDER_GIT_COMMIT` 变量在该服务配置下不可用/为空

为了让 staging 上也能看到真实版本，需要一个“平台无关”的兜底来源。

---

## Goal

当 `git_sha` 计算结果为 `"unknown"` 时，使用 **GitHub 公共 API**（无需 token）获取仓库某分支最新 commit SHA，并作为 `git_sha` 的最终兜底（取前 7 位）。

---

## Non-goals

- 不引入数据库/持久缓存
- 不引入 GitHub token（默认走匿名限流；如后续需要再加）
- 不保证绝对实时（允许短时间缓存）
- 不改变现有业务 API，仅影响 debug/build 输出

---

## 方案

### 1) 触发条件

仅当满足以下条件时尝试 GitHub API：
- `git_sha` 当前值为 `"unknown"`（或缺失）

否则（已有 vcs.revision / buildGitSHA / Render env）一律不请求 GitHub。

### 2) 目标仓库与分支解析

优先从 Render 默认环境变量获取：
- `RENDER_GIT_REPO_SLUG`（形如 `owner/repo`）
- `RENDER_GIT_BRANCH`（形如 `main`）

若任一缺失，则 fallback 到默认值：
- repo：`dengxilong2025/lobster-world-core`
- branch：`main`

### 3) 请求与解析

请求：
- `GET https://api.github.com/repos/<repo_slug>/commits/<branch>`

Headers（最小）：
- `Accept: application/vnd.github+json`
- `User-Agent: lobster-world-core`

解析：
- JSON 路径：`sha`（40位）
- 输出：`git_sha = sha[:7]`

### 4) 缓存与限流防护

在进程内维护缓存：
- key：`repo_slug + ":" + branch`
- value：`sha7`
- ttl：5 分钟（可配置常量）

策略：
- cache hit → 直接返回 sha7
- cache miss → 发起一次请求并更新缓存
- 请求失败（非 200 / JSON 解析失败 / 超时）→ 返回 `"unknown"`（不影响接口可用性）

### 5) 超时与可靠性

- HTTP client timeout：2 秒
- 不允许阻塞过久：即使 GitHub API 慢或失败，也要快速返回

---

## 输出示例

```json
{
  "ok": true,
  "build": {
    "git_sha": "91b90ce",
    "go_version": "go1.22.12",
    "module": "lobster-world-core",
    "start_time": "2026-04-26T08:12:03Z",
    "uptime_sec": 61
  }
}
```

---

## 验收标准

在 Render staging：
- `curl -sS https://lobster-world-core.onrender.com/api/v0/debug/build`
  - `build.git_sha` 不为 `"unknown"`
  - 且长度为 7

在本地（无网络/或 GitHub API 失败）：
- 仍返回 200，且 `git_sha` 至少为 `"unknown"`（不因外部依赖导致失败）

