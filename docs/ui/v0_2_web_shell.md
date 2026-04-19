# v0.2 Web 雏形（/ui）使用手册（给人类 + 给智能体）

> 目标：让测试者（包括 20 个龙虾智能体）可以用最短路径完成：  
> **提交意图 → 观战摘要 → 实时事件流（SSE）→ 打开回放（highlight）**

---

## 1) 一句话说明

打开 `/ui` 页面，填 `world_id`、写一句 `goal`，点击“提交意图”，再点“连接事件流”。  
页面会持续显示：
- 世界阶段（world.stage）
- 世界摘要（world.summary：包含“近期/看点/风险/建议”等）
- 实时事件流（SSE）
- 最近事件的回放入口（replay/highlight 链接）

---

## 2) 本地启动（开发者）

如果你用的是仓库的 server：

```bash
go run ./cmd/server
```

然后在浏览器打开（端口以 server 输出为准）：
```
http://localhost:8080/ui
```

> 备注：如果你的环境没有 `go`，可以使用项目测试环境中同样的 Go 安装方式（见 CI / 或我在云端用的安装命令）。

---

## 2.5) 一键启动（Docker Compose，推荐给非开发同学/智能体机）

在仓库根目录执行：

```bash
docker compose up --build
```

启动后打开：
```
http://localhost:8080/ui
```

停止：
```bash
docker compose down
```

---

## 3) /ui 页面元素（给自动化/智能体脚本）

页面中固定的 DOM id（后续尽量不变）：
- `world_id`：world id 输入框
- `goal`：意图 goal 输入框
- `btn_intent`：提交意图按钮
- `btn_connect`：连接 SSE 按钮
- `status`：状态提示
- `world_stage`：世界阶段展示
- `world_summary`：世界摘要列表（UL）
- `events`：事件流原始输出（PRE）
- `replays`：回放入口列表（UL，包含 `<a target="_blank">`）

---

## 4) 最短测试路径（人类）

1. 打开 `/ui`
2. 输入 `world_id`（例如 `w1`）
3. 输入 `goal`（例如 `去狩猎获取食物`）
4. 点击 **提交意图**
5. 点击 **连接事件流**
6. 等待 1~3 秒，你应该能看到：
   - `events` 开始刷出 JSON（每条是一个 event）
   - `world_summary` 列表刷新（阶段/近期/看点/风险/建议）
   - `replays` 出现一条或多条链接
7. 点击 `replays` 的链接，会打开：
   - `/api/v0/replay/highlight?world_id=...&event_id=...`

---

## 4.5) 可脚本化“直达链接”（给智能体/自动化）

`/ui` 支持 URL 参数，便于你直接把一个“可复现的测试入口链接”发给任何智能体：

### 参数说明
- `world_id`：必填，世界 ID
- `goal`：可选，预填 goal 输入框
- `autoconnect`：可选，`1` 或 `true` 时页面加载后会自动连接 SSE 并拉取 spectator/home

### 示例
1) 只预填 world_id：
```
http://localhost:8080/ui?world_id=w1
```

2) 预填 world_id + goal：
```
http://localhost:8080/ui?world_id=w1&goal=%E5%8E%BB%E7%8B%A9%E7%8C%8E%E8%8E%B7%E5%8F%96%E9%A3%9F%E7%89%A9
```

3) 自动连接（推荐给智能体批量测试）：
```
http://localhost:8080/ui?world_id=w1&autoconnect=1
```

4) 自动连接 + 预填 goal（最方便复现）：
```
http://localhost:8080/ui?world_id=w1&goal=%E5%8E%BB%E7%8B%A9%E7%8C%8E%E8%8E%B7%E5%8F%96%E9%A3%9F%E7%89%A9&autoconnect=1
```

## 5) 智能体测试路径（HTTP 版本，不依赖浏览器）

> 适合“20 个龙虾智能体”做压测/回归/体验统计。

### Step A：提交意图
`POST /api/v0/intents`

```bash
curl -sS -X POST "http://localhost:8080/api/v0/intents" \
  -H "Content-Type: application/json" \
  -d '{"world_id":"w1","goal":"去狩猎获取食物"}'
```

### Step B：订阅事件流（SSE）
`GET /api/v0/events?world_id=w1`

用 curl 观察（会持续输出）：

```bash
curl -N "http://localhost:8080/api/v0/events?world_id=w1"
```

从输出中解析 `event_id`（通常在 JSON 里）。

### Step C：拉取观战摘要
`GET /api/v0/spectator/home?world_id=w1`

```bash
curl -sS "http://localhost:8080/api/v0/spectator/home?world_id=w1" | jq .
```

你应该看到：
- `world.stage`
- `world.summary[]`（其中包含 “近期：… / 看点：… / 风险：… / 建议：…” 等行）

### Step D：打开回放
`GET /api/v0/replay/highlight?world_id=w1&event_id=<event_id>`

```bash
curl -sS "http://localhost:8080/api/v0/replay/highlight?world_id=w1&event_id=<event_id>" | jq .
```

输出中 `beats[]` 是最关键的可读“解说节奏”数据。

---

## 6) 常见问题（FAQ）

### Q1：/ui 上显示 SSE 连接中断？
SSE 断线后浏览器会自动重连；页面会显示提示文字。  
如果一直断线，检查：
- server 是否还在运行
- 是否 world_id 写错

### Q2：world.summary 为空？
需要世界至少产生一些事件（提交 intent 或等待演化 tick），再刷新。

### Q3：replays 不出现？
replays 是从 SSE event JSON 里解析 `event_id` 生成的。  
如果 SSE 输出不是 JSON 或缺少 `event_id`，说明后端事件格式有问题（应属于 P0 bug）。
