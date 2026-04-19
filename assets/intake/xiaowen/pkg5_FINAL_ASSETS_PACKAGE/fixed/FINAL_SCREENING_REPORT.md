# 龙虾世界 - 资源筛选最终报告

## 一、筛选标准（按小梦v0.2规范）

1. **尺寸规范**：瓦片256x256，UI组件按规格
2. **透明通道**：建筑/装饰/UI图标需要透明，地形瓦片可不透明
3. **命名规范**：全小写+下划线
4. **风格红线**：暖色系、无文字/水印、手绘部落风
5. **3x3拼贴测试**：瓦片需可平铺

---

## 二、精品资源清单

### 1. tiles (16个) - ✅ 精品
| 文件名 | 尺寸 | 类型 | 质量 |
|--------|------|------|------|
| tile_campsite_w256_h256.png | 256x256 | 营营地 | 待验收 |
| tile_clay_soil_w256_h256.png | 256x256 | 粘土土 | 待验收 |
| tile_dirt_01_w256_h256.png | 256x256 | 泥土 | 待验收 |
| tile_forest_w256_h256.png | 256x256 | 森林 | 待验收 |
| tile_gobi_w256_h256.png | 256x256 | 戈壁 | 待验收 |
| tile_grassland_w256_h256.png | 256x256 | 草地 | ⭐⭐⭐⭐ |
| tile_lakeshore_w256_h256.png | 256x256 | 湖岸 | 待验收 |
| tile_oasis_w256_h256.png | 256x256 | 绿洲 | 待验收 |
| tile_reed_marsh_w256_h256.png | 256x256 | 芦苇沼泽 | 待验收 |
| tile_riverside_w256_h256.png | 256x256 | 河岸 | 待验收 |
| tile_rocky_new_w256_h256.png | 256x256 | 岩石 | 待验收 |
| tile_ruins_w256_h256.png | 256x256 | 废墟 | 待验收 |
| tile_sandalwood_w256_h256.png | 256x256 | 檀木 | 待验收 |
| tile_sand_new_w256_h256.png | 256x256 | 沙地 | 待验收 |
| tile_stone_01_w256_h256.png | 256x256 | 石头 | 待验收 |
| tile_swamp_w256_h256.png | 256x256 | 沼泽 | 待验收 |

### 2. tiles_individual (104个) - ✅ 精品
- 尺寸：256x256，透明通道
- 类型：建筑、装饰、设施、自然物、工具、树木、道路、水体
- 命名：符合规范
- 质量：大部分4-5分

### 3. props_final (18个)
| 文件名 | 尺寸 | 质量 |
|--------|------|------|
| bldg_isometric_primitive_01-04 | 1024x1024 | ⭐⭐⭐⭐⭐ |
| prop_trees_tent_campfire_01-04 | 1024x1024 | 待验收 |
| prop_weapons_structures_01-04 | 1024x1024 | 待验收 |
| tileset_grass_01-04 | 1024x1024 | ⭐⭐⭐⭐⭐ |
| test.png, test2.png | 1024x1024 | ❌ 淘汰 |

### 4. ui_fixed (13个) - ✅ 精品
- badges: 4个 (256x64)
- frames: 9个 (九宫格切片)

### 5. scenes (1个) - ✅ 精品
- spectate_bg_w1024_h1024.png: ⭐⭐⭐⭐

---

## 三、需淘汰资源

| 目录 | 数量 | 原因 |
|------|------|------|
| test.png, test2.png | 2 | 测试文件 |
| LPC_tiles | 12 | 尺寸混乱 |
| tiles_new | 20 | 尺寸不符（标256实1024） |

---

## 四、预验收结论

### 通过预验收（可直接打包）
- tiles (16个): 256x256瓦片
- tiles_individual (104个): 装饰/建筑组件
- ui_fixed (13个): UI组件
- spectate_bg (1个): 观战背景

### 建议保留（需小梦确认）
- props_final等轴建筑 (4个): 1024x1024
- props_final草地tileset (4个): 1024x1024
