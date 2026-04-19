package gateway

const uiAssetsPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Lobster World - Assets Preview</title>
  <style>
    body { font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Arial; margin: 16px; color: #222; }
    header { display:flex; gap:12px; align-items:center; flex-wrap: wrap; }
    select, input { padding: 6px 8px; }
    .hint { color:#666; font-size: 12px; }
    .grid { display:grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 12px; margin-top: 16px; }
    .card { border:1px solid #ddd; border-radius:10px; padding:10px; background:#fff; }
    .thumb { width: 100%; height: 110px; display:flex; align-items:center; justify-content:center; background: #f6f6f6; border-radius: 8px; overflow:hidden; }
    .thumb img { max-width: 100%; max-height: 100%; image-rendering: auto; }
    .name { margin-top: 8px; font-size: 12px; word-break: break-all; }
    .tag { display:inline-block; font-size: 11px; padding:2px 6px; border-radius: 999px; background:#eee; margin-top:6px; color:#333; }
    .modal_backdrop { display:none; position:fixed; inset:0; background:rgba(0,0,0,0.55); z-index:9999; }
    .modal_panel { max-width:980px; margin:40px auto; background:#fff; border-radius:12px; padding:16px; box-shadow: 0 12px 40px rgba(0,0,0,0.25); }
  </style>
</head>
<body>
  <header>
    <h2 style="margin:0;">Assets Preview</h2>
    <label>分类：
      <select id="cat">
        <option value="ui.icons">ui.icons</option>
        <option value="ui.frames">ui.frames</option>
        <option value="ui.badges">ui.badges</option>
        <option value="tiles.base">tiles.base</option>
        <option value="tiles.props">tiles.props</option>
        <option value="tiles.props_final_1024">tiles.props_final_1024</option>
        <option value="scenes">scenes</option>
      </select>
    </label>
    <label>搜索：<input id="q" placeholder="输入文件名片段…" size="26"/></label>
    <span class="hint">数据来自 <code>/assets/production/manifest.json</code></span>
  </header>

  <div id="status" class="hint" style="margin-top:10px;"></div>
  <div id="grid" class="grid"></div>

  <div id="asset_modal" class="modal_backdrop">
    <div class="modal_panel">
      <div style="display:flex; justify-content:space-between; align-items:center; gap:12px;">
        <div>
          <div id="modal_title" style="font-weight:600;"></div>
          <div id="modal_meta" class="hint"></div>
        </div>
        <button id="modal_close" style="padding:6px 10px;">关闭</button>
      </div>
      <div style="display:grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-top: 12px;">
        <div>
          <div class="hint">原图预览（点击可在新标签打开）</div>
          <a id="modal_link" class="thumb" style="height:340px;" target="_blank" rel="noreferrer"><img id="modal_img" /></a>
        </div>
        <div>
          <div class="hint">3×3 拼贴预览（仅 tiles.base）</div>
          <canvas id="canvas_3x3" width="768" height="768" style="width:100%; border:1px solid #ddd; border-radius:10px; background:#777;"></canvas>
          <div class="hint" id="qc_stats" style="margin-top:8px;"></div>
        </div>
      </div>
    </div>
  </div>

  <script>
    const MANIFEST_URL = '/assets/production/manifest.json';
    const BASE_URL = '/assets/production/';

    function el(id){ return document.getElementById(id); }
    function setStatus(s){ el('status').textContent = s; }

    function normList(v){
      if (!v) return [];
      if (Array.isArray(v)) return v;
      // allow {items:[...]} shape
      if (v.items && Array.isArray(v.items)) return v.items;
      return [];
    }

    let manifest = null;
    let modalState = { cat: null, relPath: null };

    async function loadManifest(){
      setStatus('加载 manifest 中…');
      const resp = await fetch(MANIFEST_URL, { cache: 'no-store' });
      if (!resp.ok) throw new Error('manifest http ' + resp.status);
      manifest = await resp.json();
      setStatus('manifest 加载成功');
    }

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

    function computeAlphaStats(img, cat){
      // Draw into an offscreen canvas to examine alpha distribution.
      const w = img.naturalWidth || img.width;
      const h = img.naturalHeight || img.height;
      if (!w || !h) return null;

      const oc = document.createElement('canvas');
      oc.width = w;
      oc.height = h;
      const ctx = oc.getContext('2d', { willReadFrequently: true });
      ctx.clearRect(0, 0, w, h);
      ctx.drawImage(img, 0, 0, w, h);
      const data = ctx.getImageData(0, 0, w, h).data;

      let zero = 0, mid = 0, full = 0;
      const total = w * h;
      for (let i = 3; i < data.length; i += 4) {
        const a = data[i];
        if (a === 0) zero++;
        else if (a === 255) full++;
        else mid++;
      }
      const rz = zero / total;
      const rm = mid / total;
      const rf = full / total;

      let warn = '';
      if (cat === 'tiles.base' && rz > 0.05) {
        warn = '注意：该地表瓦片存在较大透明区域，可能是误抠或通道异常（类似 glitch 案例）。';
      }
      if (cat === 'ui.icons' && rf === 1.0) {
        warn = '注意：该图标可能没有透明背景（alpha 全 255）。';
      }

      return { w, h, rz, rm, rf, warn };
    }

    function closeModal(){
      el('asset_modal').style.display = 'none';
    }

    function openModal(cat, relPath){
      modalState = { cat, relPath };
      const url = BASE_URL + relPath;
      el('asset_modal').style.display = 'block';
      el('modal_title').textContent = relPath;
      el('modal_meta').textContent = '分类：' + cat;
      el('modal_link').href = url;
      el('qc_stats').textContent = '加载中…';

      const img = el('modal_img');
      img.onload = () => {
        const st = computeAlphaStats(img, cat);
        if (st) {
          const pct = (x) => Math.round(x * 1000) / 10; // 0.1%
          let msg = 'alpha 统计：透明 ' + pct(st.rz) + '%，半透明 ' + pct(st.rm) + '%，不透明 ' + pct(st.rf) + '%';
          if (st.warn) msg += '；' + st.warn;
          el('qc_stats').textContent = msg;
        } else {
          el('qc_stats').textContent = 'alpha 统计：无法计算';
        }

        if (cat === 'tiles.base') {
          draw3x3(img);
        } else {
          const c = el('canvas_3x3');
          const ctx = c.getContext('2d');
          ctx.clearRect(0,0,c.width,c.height);
          // keep qc_stats (alpha message). Just avoid implying 3×3 is available.
        }
      };
      img.onerror = () => {
        el('qc_stats').textContent = '图片加载失败';
      };
      img.src = url;
    }

    function getCategoryList(cat){
      if (!manifest) return [];
      // expected shape: { ui: { icons: [] ... }, tiles: { base: [] ... }, scenes: [] }
      if (cat === 'scenes') return normList(manifest.scenes);
      const [a,b] = cat.split('.');
      if (!a || !b) return [];
      return normList(manifest[a] && manifest[a][b]);
    }

    function render(){
      const cat = el('cat').value;
      const q = el('q').value.trim().toLowerCase();
      const list = getCategoryList(cat).filter(p => !q || p.toLowerCase().includes(q));
      const grid = el('grid');
      grid.innerHTML = '';
      setStatus('分类 ' + cat + '：' + list.length + ' 项');

      for (const relPath of list) {
        const url = BASE_URL + relPath;
        const card = document.createElement('div');
        card.className = 'card';
        card.style.cursor = 'pointer';
        card.addEventListener('click', (e) => {
          if (e.metaKey || e.ctrlKey) return;
          if (e.target && e.target.closest && e.target.closest('a')) return;
          openModal(cat, relPath);
        });

        const thumb = document.createElement('a');
        thumb.className = 'thumb';
        thumb.href = url;
        thumb.target = '_blank';
        thumb.rel = 'noreferrer';

        const img = document.createElement('img');
        img.loading = 'lazy';
        img.src = url;
        img.alt = relPath;
        thumb.appendChild(img);

        const name = document.createElement('div');
        name.className = 'name';
        name.textContent = relPath;

        const tag = document.createElement('div');
        tag.className = 'tag';
        tag.textContent = cat;

        card.appendChild(thumb);
        card.appendChild(name);
        card.appendChild(tag);
        grid.appendChild(card);
      }
    }

    (async function main(){
      try {
        await loadManifest();
        render();
        el('cat').addEventListener('change', render);
        el('q').addEventListener('input', render);
        el('modal_close').addEventListener('click', closeModal);
        el('asset_modal').addEventListener('click', (e) => {
          if (e.target === el('asset_modal')) closeModal();
        });
      } catch (e) {
        setStatus('加载失败：' + (e && e.message ? e.message : e));
      }
    })();
  </script>
</body>
</html>`
