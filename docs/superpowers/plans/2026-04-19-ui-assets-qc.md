# UI Assets QC (3×3 Tiling + Alpha Checks) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade `/ui/assets` to support (1) 3×3 tile preview for `tiles.base` and (2) on-demand alpha anomaly checks, to speed up art QA and intake validation.

**Architecture:** Keep everything client-side to avoid backend image processing. When user clicks a tile under `tiles.base`, open a modal with a Canvas-based 3×3 composite preview. For any image, compute alpha statistics on demand via Canvas pixel sampling and show a small QC badge (e.g., “alpha holes risk”) in the modal. Keep server changes minimal (only HTML/JS updates).

**Tech Stack:** Go (existing gateway HTML embedding), plain HTML/JS, Canvas 2D API.

---

## File/Module Map

**Modify**
- `internal/gateway/ui_assets_page.go` — add modal UI + 3×3 canvas + QC logic
- `tests/integration/ui_assets_smoke_test.go` — extend assertions so we know QC UI is present

---

## Task 1: Add failing test asserting QC UI hooks exist (TDD)

**Files:**
- Modify: `tests/integration/ui_assets_smoke_test.go`

- [ ] **Step 1: Update `TestUIAssets_ServesHTML` to require QC markers**

In `TestUIAssets_ServesHTML`, add assertions that will fail until QC UI is implemented:

```go
if !strings.Contains(body, "id=\"asset_modal\"") {
  t.Fatalf("expected assets page contains #asset_modal")
}
if !strings.Contains(body, "id=\"canvas_3x3\"") {
  t.Fatalf("expected assets page contains #canvas_3x3")
}
```

- [ ] **Step 2: Run the test to confirm it FAILS**

Run:
```bash
go test ./... -run TestUIAssets_ServesHTML -v
```

Expected: FAIL because the HTML does not yet contain those ids.

- [ ] **Step 3: Commit failing test**

```bash
git add tests/integration/ui_assets_smoke_test.go
git commit -m "test(ui): require qc modal and 3x3 canvas in /ui/assets"
```

---

## Task 2: Implement modal + 3×3 tiling preview for `tiles.base`

**Files:**
- Modify: `internal/gateway/ui_assets_page.go`

- [ ] **Step 1: Add modal markup**

Add below the grid container:

```html
<div id="asset_modal" style="display:none; position:fixed; inset:0; background:rgba(0,0,0,0.55); z-index:9999;">
  <div style="max-width:980px; margin:40px auto; background:#fff; border-radius:12px; padding:16px;">
    <div style="display:flex; justify-content:space-between; align-items:center; gap:12px;">
      <div>
        <div id="modal_title" style="font-weight:600;"></div>
        <div id="modal_meta" class="hint"></div>
      </div>
      <button id="modal_close" style="padding:6px 10px;">关闭</button>
    </div>
    <div style="display:grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-top: 12px;">
      <div>
        <div class="hint">原图预览</div>
        <div class="thumb" style="height:340px;"><img id="modal_img" /></div>
      </div>
      <div>
        <div class="hint">3×3 拼贴预览（仅 tiles.base）</div>
        <canvas id="canvas_3x3" width="768" height="768" style="width:100%; border:1px solid #ddd; border-radius:10px; background:#777;"></canvas>
        <div class="hint" id="qc_stats" style="margin-top:8px;"></div>
      </div>
    </div>
  </div>
</div>
```

- [ ] **Step 2: Make each card clickable (open modal)**

When rendering cards, attach `onclick` with `relPath` and current `cat`.

- [ ] **Step 3: Implement `render3x3Tile(image)`**

JS function:
1) Clear canvas
2) Draw the same tile 9 times (3×3), each at 256×256, filling the 768×768 canvas

```js
function draw3x3(img){
  const c = el('canvas_3x3');
  const ctx = c.getContext('2d');
  ctx.clearRect(0,0,c.width,c.height);
  const sz = 256;
  for (let y=0;y<3;y++){
    for (let x=0;x<3;x++){
      ctx.drawImage(img, x*sz, y*sz, sz, sz);
    }
  }
}
```

Only call `draw3x3` when category is `tiles.base`; otherwise show a message in `qc_stats` like “该分类不做拼贴预览” and leave canvas blank.

- [ ] **Step 4: Run tests and confirm `TestUIAssets_ServesHTML` now passes**

Run:
```bash
go test ./... -run TestUIAssets_ServesHTML -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/ui_assets_page.go
git commit -m "feat(ui): add asset modal and 3x3 tiling preview for tiles.base"
```

---

## Task 3: Add on-demand alpha anomaly checks (client-side)

**Files:**
- Modify: `internal/gateway/ui_assets_page.go`

- [ ] **Step 1: Compute alpha coverage stats**

In modal open flow, after image loads, draw it to an offscreen canvas (256×256 for tiles; for icons keep natural size).
Sample pixels and compute:
- `alphaZeroRatio = count(alpha==0)/total`
- `alphaMidRatio = count(0<alpha<255)/total`
- `alphaFullRatio = count(alpha==255)/total`

Show in `#qc_stats`.

- [ ] **Step 2: Heuristic warning**

If `tiles.base` and `alphaZeroRatio > 0.05`, show warning:
> “⚠️ 该地表瓦片存在较大透明区域，可能是误抠/通道异常（类似之前的 glitch 案例）”

If `ui.icons` and `alphaFullRatio == 1.0`, show warning:
> “⚠️ icon 可能没有透明背景（alpha 全 255）”

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/ui_assets_page.go
git commit -m "feat(ui): add alpha qc stats and warnings in asset modal"
```

---

## Task 4: Manual verification checklist

- [ ] Start server: `go run ./cmd/server`
- [ ] Open `http://localhost:8080/ui/assets`
- [ ] Switch to `tiles.base`, click a tile → should see 3×3 preview
- [ ] Switch to `ui.icons`, click an icon → should see qc stats and no 3×3 preview
- [ ] Try searching and opening an image in new tab still works

---

## Spec Self-Review Checklist

- No TODO/TBD placeholders
- Tests fail first, then pass after implementation
- QC features are limited to `tiles.base` as requested
- Changes remain client-side (no server image processing)

