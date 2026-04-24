# Render Staging（MVP）部署说明

> 目标：用 Render（Hobby/Free）快速部署一个可对外访问的 staging URL，用于演示与外测。

## 1) Render 服务配置（建议）

- 类型：Web Service
- Runtime：Docker（使用仓库根目录 `Dockerfile`）
- Branch：`main`
- Health Check Path：`/healthz`

## 2) Smoke 验收

把 `<STAGING_URL>` 替换为你的 Render URL：

```bash
curl -sS -I "<STAGING_URL>/healthz" | head -n 1
curl -sS -I "<STAGING_URL>/ui" | head -n 1
curl -sS -I "<STAGING_URL>/assets/production/manifest.json" | head -n 1
```

## 3) 注意事项

- 免费层可能休眠/冷启动：第一次访问慢是正常现象。
- 当前为内存态：重启/重新部署会清空 world 状态与事件。

