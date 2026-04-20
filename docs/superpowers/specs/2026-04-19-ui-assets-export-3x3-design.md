# /ui/assets 3×3 拼贴导出（PNG）设计说明

**背景/动机**
- 目前 `/ui/assets` 已支持 `tiles.base` 的 3×3 Canvas 拼贴预览，用于肉眼检查“接缝/重复感/通道异常”。
- 需要新增“一键导出拼贴图”为 PNG 的能力，用于**验收留档**与异步沟通（给美术/策划/管理侧看）。

---

## 目标（P0/P1）

**P0（本次必须交付）**
1) 仅对 `tiles.base` 分类：在资产弹窗中提供 **“导出 3×3 PNG”** 按钮
2) 点击后直接在浏览器端下载一张 **768×768 PNG**（即当前 Canvas 内容）
3) 导出图片为**纯图片**（不加任何文字水印/标注）
4) 文件名包含 tile 文件名 + 3×3 标识 + 时间戳，方便检索与留档

**P1（可选/后续）**
- 支持批量导出（一次导出某分类全部 tiles.base 的拼贴图）
- 支持导出“带文字标注”的版本（角落写 tile 名/时间戳），但注意这会在图上出现文字

---

## 方案选择

采用 **方案 1：前端直接从 Canvas 导出（推荐）**：
- 使用 `canvas.toBlob()` 或 `canvas.toDataURL()` 生成 PNG
- 通过创建临时 `<a download>` 触发浏览器下载
- 不引入后端图片处理，不产生服务器存储/权限/持久化问题

---

## 交互与行为

### 入口
- `/ui/assets` 的资产弹窗（已存在）
- 仅当当前分类为 `tiles.base` 时显示导出按钮；否则置灰或隐藏，并提示“仅 tiles.base 支持拼贴导出”

### 导出内容
- 使用弹窗中的 `canvas_3x3`，导出其当前绘制内容
- 图片尺寸：`768×768`
- 图片格式：`PNG`
- 图片内容：纯拼贴画面（不叠加任何文字/水印）

### 文件命名
```
{basename}__3x3__YYYYMMDD-HHMMSS.png
```
示例：
- `tile_grass_basic_01_w256_h256__3x3__20260419-213012.png`

其中 `{basename}` 为 `relPath` 的文件名（去掉目录与扩展名）。

---

## 实现要点（工程）

### 前端（HTML/JS，嵌入在 `internal/gateway/ui_assets_page.go`）
- 增加按钮 DOM：`id="btn_export_3x3"`
- 在 `openModal(cat, relPath)` 时：
  - 若 `cat === 'tiles.base'`：按钮可点击
  - 否则：按钮禁用
- 点击导出：
  - 从 `canvas_3x3` 获取 Blob：`canvas.toBlob((blob) => { ... }, 'image/png')`
  - `URL.createObjectURL(blob)` + `<a download>` 触发下载
  - 释放 URL：`URL.revokeObjectURL(url)`

### 后端
不需要新增 API；复用现有 `/ui/assets` 页面与静态资源服务。

---

## 测试策略

**自动化（集成测试）**
- 扩展现有 `TestUIAssets_ServesHTML`：
  - 断言页面包含 `id="btn_export_3x3"`（可保证入口不会被误删）

**手工验收**
1) 启动服务 `go run ./cmd/server`
2) 打开 `http://localhost:8080/ui/assets`
3) 切到 `tiles.base`，点击任意瓦片打开弹窗
4) 点击“导出 3×3 PNG”，确认浏览器下载一张 768×768 PNG，文件名符合规则

---

## 风险与约束
- 浏览器下载行为可能被某些安全设置限制（但一般本地/常规浏览器不会）
- 如果 Canvas 未完成绘制就导出，可能导出空白；因此按钮应在图片 `onload` 且已绘制 3×3 后启用

