# 资源包盘点报告（5/7）— FINAL_ASSETS_PACKAGE.zip

> 这包的组织和质量明显更接近“可交付版本”：自带 `FINAL_SCREENING_REPORT.md`，目录也非常干净。  
> 主要问题只剩一个：`tiles_individual/` 和 `props_final/` 仍存在“内部亮洞（高光被抠透明）”的常见副作用（我已批量修复一版）。

---

## 1) 包结构与数量统计

文件总数：**151**
- PNG：150
- 文档：`FINAL_SCREENING_REPORT.md`（1）

目录：
- `tiles/`：16（256×256）
- `tiles_individual/`：104（256×256）
- `ui/`：13（badges+frames）
- `props_final/`：16（1024×1024，注：筛选报告里提到 18 个，但实际包内是 16 个）
- `scenes/`：1（`spectate_bg_w1024_h1024.png`）

尺寸分布（Top）：
- 256×256：121
- 1024×1024：17
- 其它（256×64、32×32、32×128、128×32）：少量

格式/透明：
- 全部 PNG 为 `RGBA`
- 其中 **alpha 全 255（等同不透明）**：16 张（全部在 `tiles/`，对地表瓦片是可接受的）
- 命名尺寸一致性：0 mismatch

---

## 2) 可用性分级（面向 v0.2 先跑起来）

### ✅ 可直接采纳
- `ui/`（13 张）：面板切片 + badges，尺寸与命名规范，风格一致
- `scenes/spectate_bg_w1024_h1024.png`：可作为观战页背景（但需要你确认是否要在前端实际使用）
- `tiles/`（16 张）：作为“场景块/地貌块”可先用；但它们并不是严格意义上的 P0 Type A（grass/dirt/sand/rock）无缝纹理瓦片

### ⚠️ 建议使用“我修复版”
- `tiles_individual/`（104 张）  
- `props_final/`（16 张）

原因：大量资源存在“抠图过度导致内部亮细节透明洞”。  
我已做保守修复（仅补封闭洞且 RGB 很亮的区域 + 去边缘黑晕），不会把背景粘回去。

修复统计（自动）：
- `tiles_individual/`：修到 63 张存在内部亮洞（补回像素 819）
- `ui/`：修到 2 张存在内部亮洞（补回像素 866）
- `props_final/`：16 张全部存在不同程度内部亮洞（补回像素 62605）

---

## 3) 你包内自带筛选报告的核对

`FINAL_SCREENING_REPORT.md` 的总体结论与我这边扫描基本一致。  
我这里补充两点“工程侧视角”：
1) `tiles/` 的 16 张 **alpha 全 255** 属于正常现象（地表不透明可接受），不是 bug  
2) `props_final` 实际包内是 16 个（不含 test.png/test2.png），这点你们内部清单可同步修正

---

## 4) 下一步（入库策略）

我会把：
- 修复版（subset）入 `assets/intake/xiaowen/pkg5_FINAL_ASSETS_PACKAGE/fixed/`
- 其中“确定可用”的 UI 与 tiles_individual/props（若不与现有 production 冲突）更新到 `assets/production/`

并 push 到 GitHub main（仍不提交原始 zip）。

