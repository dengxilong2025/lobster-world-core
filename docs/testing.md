# 测试与验收（v0）

## 运行
```bash
go test ./...
```

## 覆盖面（建议第三方审计重点看）
- 单元测试：`internal/**`
  - store：唯一性、查询、GetByID
  - sim：stop、背压、shock、determinism、intent rules、evolution
  - spectator：排序/热度/关系推导
- 集成测试：`tests/integration/**`
  - determinism（同 seed 同输入 -> 事件序列与状态一致）
  - SSE resume
  - replay/highlight/export
  - spectator realtime

## 回归准则（每阶段结束）
- `go test ./...` 必须全绿
- determinism 类测试必须稳定（若 flaky，优先修复而不是跳过）
- 对外行为变更必须由测试覆盖（尤其是错误码：429/503 等）

