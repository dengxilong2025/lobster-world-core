package gateway

// uiPageHTML is a minimal single-page web shell for v0.2-M1.
//
// Notes:
// - This page is served by the Go backend (no extra frontend build tool).
// - Page text is allowed; the "no text" rule applies to art/image assets only.
// - Keep DOM ids stable so agentic testers can script interactions.
const uiPageHTML = `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Lobster World / v0 UI</title>
    <style>
      body { font-family: ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial; margin: 24px; color: #111; }
      .row { display: flex; gap: 12px; flex-wrap: wrap; align-items: center; }
      input, button { padding: 8px 10px; font-size: 14px; }
      button { cursor: pointer; }
      pre { background: #f6f6f6; padding: 12px; border-radius: 8px; overflow: auto; max-height: 40vh; }
      ul { margin: 8px 0; padding-left: 20px; }
      .muted { color: #666; }
    </style>
  </head>
  <body>
    <h1>Lobster World（v0.2 Web 雏形）</h1>
    <p class="muted">目标：提交意图 → 观战摘要 → 实时事件流（SSE）→ 回放入口</p>

    <div class="row">
      <label>world_id：<input id="world_id" placeholder="例如 w1" value="w1" /></label>
      <label>goal：<input id="goal" placeholder="例如 去狩猎获取食物" size="32" /></label>
      <button id="btn_intent">提交意图</button>
      <button id="btn_connect">连接事件流</button>
      <button id="btn_export">导出回放（NDJSON）</button>
      <span id="status" class="muted"></span>
    </div>

    <h2>世界阶段</h2>
    <div id="world_stage" class="muted">（未加载）</div>

    <h2>世界摘要</h2>
    <ul id="world_summary"></ul>

    <h2>事件流（最新）</h2>
    <pre id="events"></pre>

    <h2>回放入口（最近事件）</h2>
    <ul id="replays"></ul>

    <script>
      // Endpoints (keep as literal strings for tests)
      const API_INTENTS = '/api/v0/intents';
      const API_EVENTS_STREAM = '/api/v0/events/stream';
      const API_HOME    = '/api/v0/spectator/home';
      const API_HIGHLIGHT = '/api/v0/replay/highlight';
      const API_EXPORT = '/api/v0/replay/export';

      const $ = (id) => document.getElementById(id);
      const statusEl = $('status');
      const eventsEl = $('events');
      const stageEl = $('world_stage');
      const summaryEl = $('world_summary');
      const replaysEl = $('replays');

      let es = null;
      let lastEvents = [];

      function setStatus(s) { statusEl.textContent = s; }

      async function fetchHome(worldId) {
        const resp = await fetch(API_HOME + '?world_id=' + encodeURIComponent(worldId));
        if (!resp.ok) throw new Error('home status ' + resp.status);
        const data = await resp.json();
        const world = data.world || null;
        stageEl.textContent = world ? world.stage : '（无）';

        // summary is []string
        summaryEl.innerHTML = '';
        const summary = (world && world.summary) ? world.summary : [];
        for (const line of summary) {
          const li = document.createElement('li');
          li.textContent = line;
          summaryEl.appendChild(li);
        }
      }

      function renderEvents() {
        eventsEl.textContent = lastEvents.join('\\n');
      }

      function addReplayLink(worldId, eventId, label) {
        const li = document.createElement('li');
        const a = document.createElement('a');
        a.href = API_HIGHLIGHT + '?world_id=' + encodeURIComponent(worldId) + '&event_id=' + encodeURIComponent(eventId);
        a.target = '_blank';
        a.rel = 'noreferrer';
        a.textContent = label || ('replay ' + eventId);
        li.appendChild(a);
        replaysEl.insertBefore(li, replaysEl.firstChild);
        // keep only recent N
        while (replaysEl.children.length > 20) replaysEl.removeChild(replaysEl.lastChild);
      }

      async function postIntent(worldId, goal) {
        const resp = await fetch(API_INTENTS, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ world_id: worldId, goal })
        });
        if (!resp.ok) throw new Error('intent status ' + resp.status);
        return resp.json();
      }

      function connectSSE(worldId) {
        if (es) { es.close(); es = null; }
        const url = API_EVENTS_STREAM + '?world_id=' + encodeURIComponent(worldId);
        es = new EventSource(url);

        es.onopen = () => setStatus('SSE 已连接');
        es.onerror = () => setStatus('SSE 连接中断（会自动重连）');
        es.onmessage = (ev) => {
          // best-effort: keep recent N lines, show raw data for debugging
          lastEvents.push(ev.data);
          if (lastEvents.length > 200) lastEvents = lastEvents.slice(-200);
          renderEvents();
          // best-effort parse to expose replay entrypoint
          try {
            const obj = JSON.parse(ev.data);
            if (obj && obj.event_id) {
              const title = (obj.narrative || obj.type || 'event') + '（' + obj.event_id + '）';
              addReplayLink(worldId, obj.event_id, title);
            }
          } catch (_) {}
          // refresh summary after events (best-effort)
          fetchHome(worldId).catch(() => {});
        };
      }

      $('btn_intent').onclick = async () => {
        const worldId = $('world_id').value.trim();
        const goal = $('goal').value.trim();
        if (!worldId || !goal) { setStatus('world_id / goal 不能为空'); return; }
        setStatus('提交中...');
        try {
          await postIntent(worldId, goal);
          setStatus('已提交意图');
          fetchHome(worldId).catch(() => {});
        } catch (e) {
          setStatus('提交失败：' + e.message);
        }
      };

      $('btn_connect').onclick = () => {
        const worldId = $('world_id').value.trim();
        if (!worldId) { setStatus('world_id 不能为空'); return; }
        setStatus('连接中...');
        connectSSE(worldId);
        fetchHome(worldId).catch(() => {});
      };

      $('btn_export').onclick = () => {
        const worldId = $('world_id').value.trim();
        if (!worldId) { setStatus('world_id 不能为空'); return; }
        const url = API_EXPORT + '?world_id=' + encodeURIComponent(worldId);
        window.open(url, '_blank', 'noreferrer');
      };

      // Scriptable params for agentic testing:
      // /ui?world_id=w1&goal=...&autoconnect=1
      (function initFromQuery() {
        try {
          const qs = new URLSearchParams(window.location.search);
          const wid = (qs.get('world_id') || '').trim();
          const goal = (qs.get('goal') || '').trim();
          const autoconnect = (qs.get('autoconnect') || '').trim();
          if (wid) $('world_id').value = wid;
          if (goal) $('goal').value = goal;
          if (wid && (autoconnect === '1' || autoconnect === 'true')) {
            connectSSE(wid);
            fetchHome(wid).catch(() => {});
          }
        } catch (_) {}
      })();
    </script>
  </body>
</html>`;
