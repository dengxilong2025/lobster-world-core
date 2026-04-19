# game_assets - 生产库 (Production v0.2)

> 版本: v0.2
> 建立日期: 2026-04-19
> 更新: 2026-04-19 (Type A瓦片修正)
> 风格: 石器时代·楼兰手绘等轴
> 铁律: 禁止任何文字/数字/水印/Logo

---

## 📁 目录结构

```
game_assets/
├── ui/
│   ├── icons/      # UI图标 (64x64 + 128x128双版本)
│   ├── frames/     # 九宫格边框切片
│   └── badges/     # 徽章/标签底图
├── tiles/          # 地形瓦片 (256x256透明/可平铺)
├── scenes/         # 场景背景
├── characters/     # 角色立绘 (待制作)
├── buildings/      # 建筑 (待制作)
└── props/          # 道具 (待制作)
```

---

## 📊 P0交付清单 (小梦验收通过后进入生产库)

### ✅ UI图标 (ui/icons/) - 12个 × 2尺寸 = 24文件
每个图标有64x64主版本和128x128高清版本

| 图标名 | 用途 |
|--------|------|
| ui_icon_intent | 意图 |
| ui_icon_replay | 重放 |
| ui_icon_event | 事件 |
| ui_icon_shock_warning | 冲击警告 |
| ui_icon_shock_started | 冲击开始 |
| ui_icon_shock_ended | 冲击结束 |
| ui_icon_betrayal | 背叛 |
| ui_icon_alliance | 联盟 |
| ui_icon_evolve | 进化 |
| ui_icon_food | 食物 |
| ui_icon_wood | 木材 |
| ui_icon_stone | 石材 |

### ✅ 九宫格切片 (ui/frames/) - 9个
| 文件名 | 尺寸 | 用途 |
|--------|------|------|
| ui_frame_panel_main_corner_tl_w32_h32.png | 32x32 | 左上角 |
| ui_frame_panel_main_corner_tr_w32_h32.png | 32x32 | 右上角 |
| ui_frame_panel_main_corner_bl_w32_h32.png | 32x32 | 左下角 |
| ui_frame_panel_main_corner_br_w32_h32.png | 32x32 | 右下角 |
| ui_frame_panel_main_edge_top_w128_h32.png | 128x32 | 上边框 |
| ui_frame_panel_main_edge_bottom_w128_h32.png | 128x32 | 下边框 |
| ui_frame_panel_main_edge_left_w32_h128.png | 32x128 | 左边框 |
| ui_frame_panel_main_edge_right_w32_h128.png | 32x128 | 右边框 |
| ui_frame_panel_main_fill_w256_h256.png | 256x256 | 中心填充 |

### ✅ Badge底图 (ui/badges/) - 4个
| 文件名 | 尺寸 | 用途 |
|--------|------|------|
| ui_badge_stage_w256_h64.png | 256x64 | 阶段标签 |
| ui_badge_risk_w256_h64.png | 256x64 | 风险标签 |
| ui_badge_advice_w256_h64.png | 256x64 | 建议标签 |
| ui_badge_hook_w256_h64.png | 256x64 | 看点标签 |

### ✅ Type A 地形瓦片 (tiles/) - 3个 (可平铺纹理)
| 文件名 | 尺寸 | 用途 |
|--------|------|------|
| tile_grass_basic_01_w256_h256.png | 256x256 | 草地基础瓦片 |
| tile_sand_basic_01_w256_h256.png | 256x256 | 沙地基础瓦片 |
| tile_stone_basic_01_w256_h256.png | 256x256 | 岩地基础瓦片 |

**瓦片说明**：Type A纹理瓦片，可无缝平铺，3×3拼接无接缝

### ✅ 场景 (scenes/) - 1个
| 文件名 | 尺寸 | 用途 |
|--------|------|------|
| spectate_bg_w1024_h1024.png | 1024x1024 | 观战背景 |

---

## 📐 规格总表

| 类型 | 生产尺寸 | 格式 | 透明 |
|------|----------|------|------|
| UI图标(主) | 64x64 | PNG | 是 |
| UI图标(高清) | 128x128 | PNG | 是 |
| 九宫格切片 | 32x32/128x32/256x256 | PNG | 是 |
| Badge底图 | 256x64 | PNG | 是 |
| Type A瓦片 | 256x256 | PNG | 是 |
| 场景背景 | 1024x1024 | PNG | 否 |

---

## 🎨 色彩规范

| 颜色 | 色值 | 用途 |
|------|------|------|
| 土黄 | #C4A35A | 主色调 |
| 深褐 | #4A3728 | 线条/边框 |
| 篝火橙 | #E67E22 | 强调色 |
| 沙色 | #E8D4A8 | 沙漠/羊皮纸 |
| 枯黄 | #D4A84B | 丰收点缀 |
| 草绿 | #6B8E23 | 植物点缀 |
| 夜空蓝 | #1A1A2E | 夜景少量 |
| 血迹红 | #8B0000 | 战斗点缀 |

---

## ❌ 禁止事项

- 任何文字/数字/水印/Logo
- 写实摄影感
- 赛博朋克/霓虹发光
- 日式动漫风格
- 冷色调为主

---

## 📋 后续计划 (P1/P2)

### P1 待制作
- [ ] Type B 带地物瓦片 (草地+石块/沙地+兽骨)
- [ ] 12个建筑相关图标
- [ ] 角色立绘 (战士/弓手/萨满/猎人/工人)

### P2 待制作
- [ ] 更多场景背景
- [ ] 建筑单体立绘
- [ ] 道具图标

---

## 🔄 交付节奏

1. P0: 基础组件 → 小梦验收 → 合格后进生产库
2. P1: 进阶资源 → 小梦验收 → 合格后进生产库
3. P2: 完善资源 → 小梦验收 → 合格后进生产库

---

*最后更新: 2026-04-19 by 小文 🦞*
