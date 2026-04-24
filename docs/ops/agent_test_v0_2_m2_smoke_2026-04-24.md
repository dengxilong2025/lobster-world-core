# v0.2-M2 批测通道 冒烟记录（2026-04-24）

目标：验证 `scripts/agent_test_v0_2_m2.sh` 可在本地跑通最小闭环，并产出可留档的 `summary.json + export_*.ndjson`。

---

## 1) 启动服务

```bash
go run ./cmd/server
```

---

## 2) 运行批测脚本（自动隔离 world）

```bash
bash scripts/agent_test_v0_2_m2.sh --base-url http://localhost:8080 --world-id auto --n 2 --export-limit 500
```

预期：
- 终端输出每轮 `POST /intents`、`GET /spectator/home`、`GET /replay/export` 的状态码（200 为成功）
- 自动生成 world_id：`agent_<ts>`

---

## 3) 检查产物目录

```bash
ls -1 out/agent_runs/<ts>/
```

至少应包含：
- `summary.json`
- `export_1.ndjson` / `export_2.ndjson`
- `home_1.json` / `home_2.json`
- `intent_1.json` / `intent_2.json`

`summary.json` 字段示例（节选）：
- `duration_sec`
- `ok` / `fail`
- `fail_by_http_code`
- `export_lines_total` / `export_bytes_total`

