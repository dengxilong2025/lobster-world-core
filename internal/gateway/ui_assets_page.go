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

    async function loadManifest(){
      setStatus('加载 manifest 中…');
      const resp = await fetch(MANIFEST_URL, { cache: 'no-store' });
      if (!resp.ok) throw new Error('manifest http ' + resp.status);
      manifest = await resp.json();
      setStatus('manifest 加载成功');
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
      } catch (e) {
        setStatus('加载失败：' + (e && e.message ? e.message : e));
      }
    })();
  </script>
</body>
</html>`
