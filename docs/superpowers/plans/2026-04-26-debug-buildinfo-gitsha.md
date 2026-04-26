# A2.1：强保证 debug/build 输出 git_sha Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `GET /api/v0/debug/build` 稳定输出短 SHA（7 位）字段 `git_sha`：优先使用 buildvcs 的 `vcs.revision`，否则使用构建时 ldflags 注入的 `buildGitSHA`，再否则输出 `"unknown"`（保证非空）。

**Architecture:** 在 gateway 的 buildinfo 逻辑中加入 `buildGitSHA` 兜底；Dockerfile builder 阶段计算 `git rev-parse --short HEAD`（取不到则降级），并用 `-ldflags -X` 注入到 `lobster-world-core/internal/gateway.buildGitSHA`。新增集成测试门禁确保 `git_sha` 永远非空。

**Tech Stack:** Go（`runtime/debug`、`time`）、Dockerfile、integration tests（httptest）。

---

## 0) Files（锁定）

**Modify:**
- `internal/gateway/buildinfo.go`
- `tests/integration/debug_buildinfo_test.go`
- `Dockerfile`

---

## Task 1: TDD — 扩展集成测试（RED）

**Files:**
- Modify: `tests/integration/debug_buildinfo_test.go`

- [ ] **Step 1: 修改测试，强制要求 git_sha 必须存在且非空**

将现有测试中的 optional 部分替换为 required：

```go
// Required: git_sha must always be non-empty (best-effort real sha, fallback "unknown").
v, ok := out.Build["git_sha"]
if !ok {
  t.Fatalf("missing git_sha")
}
s, _ := v.(string)
if s == "" {
  t.Fatalf("git_sha empty")
}
```

- [ ] **Step 2: 运行测试确认失败（RED）**
```bash
go test ./tests/integration -run TestDebugBuild_ReturnsBuildInfo -v
```
Expected: FAIL（当前可能不返回 git_sha）

- [ ] **Step 3: Commit（仅测试）**
```bash
git add tests/integration/debug_buildinfo_test.go
git commit -m "test(debug): require git_sha in /debug/build"
```

---

## Task 2: 代码兜底（GREEN）

**Files:**
- Modify: `internal/gateway/buildinfo.go`

- [ ] **Step 1: 增加可注入变量**

在 `internal/gateway/buildinfo.go` 顶部加入：
```go
// buildGitSHA is injected at build time (Dockerfile ldflags). It should be a short SHA (7 chars).
// If empty, buildInfoSnapshot falls back to buildvcs (vcs.revision) or "unknown".
var buildGitSHA string
```

- [ ] **Step 2: 在 buildInfoSnapshot 中兜底输出 git_sha**

实现策略：
1) 先尝试从 buildvcs 设置中取 `vcs.revision`（现有逻辑）
2) 如果没取到且 `buildGitSHA != ""`：`out["git_sha"]=buildGitSHA`
3) 如果仍没取到：`out["git_sha"]="unknown"`

代码片段（按实际文件上下文嵌入）：
```go
// After parsing debug.ReadBuildInfo settings:
if _, ok := out["git_sha"]; !ok || out["git_sha"] == "" {
  if buildGitSHA != "" {
    out["git_sha"] = buildGitSHA
  } else {
    out["git_sha"] = "unknown"
  }
}
```

- [ ] **Step 3: 跑测试转绿**
```bash
go test ./tests/integration -run TestDebugBuild_ReturnsBuildInfo -v
go test ./...
```
Expected: PASS

- [ ] **Step 4: Commit（代码兜底）**
```bash
git add internal/gateway/buildinfo.go
git commit -m "fix(debug): always provide git_sha in build info"
```

---

## Task 3: Dockerfile 注入短 SHA（Render 强保证）

**Files:**
- Modify: `Dockerfile`

- [ ] **Step 1: 修改 builder 阶段 build 命令**

将：
```dockerfile
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/server ./cmd/server
```

替换为（注意：取不到 git 时不要失败）：
```dockerfile
RUN set -e; \
  GIT_SHA="$(git rev-parse --short HEAD 2>/dev/null || true)"; \
  if [ -z "$GIT_SHA" ]; then GIT_SHA="unknown"; fi; \
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath \
    -ldflags "-s -w -X lobster-world-core/internal/gateway.buildGitSHA=$GIT_SHA" \
    -o /out/server ./cmd/server
```

- [ ] **Step 2: 本地 docker build 验证（可选但推荐）**
```bash
docker build -t lobster-world-core:dev .
docker run --rm -p 8080:8080 lobster-world-core:dev
curl -sS http://localhost:8080/api/v0/debug/build | head -c 300; echo
```
Expected: `git_sha` 非空，通常为 7 位；若 build context 无 `.git` 则为 `"unknown"`。

- [ ] **Step 3: Commit（Dockerfile）**
```bash
git add Dockerfile
git commit -m "build(docker): inject short git sha into binary"
```

---

## Task 4: 部署与线上验收（Render）

- [ ] **Step 1: push main 触发 Render 自动部署**
```bash
git push origin main
```

- [ ] **Step 2: staging 验收**
```bash
curl -sS "https://lobster-world-core.onrender.com/api/v0/debug/build" | head -c 500; echo
```
Expected: `build.git_sha` 存在且非空（理想情况下为 7 位 commit）。

