# A1：staging smoke 脚本（Render）Design（2026-04-26）

## Goal

提供一个“一条命令即可判断 staging 是否可用”的脚本，用于：
- 日常部署后快速验收（尤其是免费实例冷启动/偶发失败）
- 外测时快速判断是“服务挂了”还是“只是 / 路径不对”
- 为 benchmarks/agent_batch 之前提供轻量前置检查（fail-fast）

## Non-goals

- 不在 smoke 中跑完整 `agent_test_v0_2_m2.sh`（避免对免费实例施压）
- 不做高并发/压测（那是 benchmarks 的职责）

---

## 1) 脚本位置与使用方式

新增：
- `scripts/smoke_staging.sh`

用法：
```bash
# 默认跑 Render staging
bash scripts/smoke_staging.sh

# 自定义目标环境
BASE_URL=http://localhost:8080 bash scripts/smoke_staging.sh
```

默认：
- `BASE_URL=https://lobster-world-core.onrender.com`

---

## 2) 检查项（顺序即 fail-fast 顺序）

### 2.1 只读可达性

1) `GET /healthz`：期望 200
2) `GET /`：期望 302，`Location: /ui`（不跟随重定向）
3) `GET /ui`：期望 200（HTML）
4) `GET /assets/production/manifest.json`：期望 200

### 2.2 最小闭环（轻量写入）

5) `POST /api/v0/intents`
   - `world_id=smoke_<ts>`
   - `goal="staging smoke"`
   - 期望 200 且响应体包含 `"ok": true`
6) `GET /api/v0/spectator/home?world_id=...`：期望 200
7) `GET /api/v0/replay/export?world_id=...&limit=200`：期望 200（内容可为空，但必须是 200）

---

## 3) 输出与退出码

- 每一步输出一行：`[OK]` / `[FAIL]` + endpoint + http_code（必要时打印 body head）
- 任一步失败：exit code = 1
- 全部通过：exit code = 0

---

## 4) 安全性与幂等

- world_id 带时间戳，避免与现有世界冲突
- export limit 小（200），避免拉取过多数据

