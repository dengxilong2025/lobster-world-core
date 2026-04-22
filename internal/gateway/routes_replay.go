package gateway

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/projections/spectator"
	"lobster-world-core/internal/sim"
)

type neighborStore interface {
	GetNeighbors(worldID, eventID string, radius int) (prev, next spec.Event, okPrev, okNext bool, err error)
}

func registerReplayRoutes(mux *http.ServeMux, es store.EventStore, sp *spectator.Projection, sm *sim.Engine) {
	// Replay highlight (MVP): return a structured "script replay" for 30s.
	mux.HandleFunc("GET /api/v0/replay/highlight", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		worldID := q.Get("world_id")
		eventID := q.Get("event_id")
		if worldID == "" || eventID == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "world_id and event_id are required")
			return
		}
		target, ok, err := es.GetByID(worldID, eventID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "event not found")
			return
		}

		// Find neighbor events to give replay context (MVP narration).
		var prev, next *spec.Event
		if ns, ok := es.(neighborStore); ok {
			p, n, okPrev, okNext, nerr := ns.GetNeighbors(worldID, eventID, 1)
			if nerr == nil {
				if okPrev {
					tmp := p
					prev = &tmp
				}
				if okNext {
					tmp := n
					next = &tmp
				}
			}
		} else {
			events, err := es.Query(store.Query{WorldID: worldID, SinceTs: 0, Limit: 1000})
			if err != nil {
				writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
				return
			}
			for i := range events {
				if events[i].EventID == eventID {
					if i > 0 {
						prev = &events[i-1]
					}
					if i+1 < len(events) {
						next = &events[i+1]
					}
					break
				}
			}
		}

		beats := buildReplayBeats(worldID, target, prev, next, es, sp, sm)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"replay_id":    "rp_" + target.EventID,
			"event_id":     target.EventID,
			"duration_sec": 30,
			"beats":        beats,
		})
	})

	// Replay export (MVP): export canonical event log as NDJSON for deterministic replay/debugging.
	// Output is sorted by (ts asc, event_id asc).
	mux.HandleFunc("GET /api/v0/replay/export", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		worldID := q.Get("world_id")
		if worldID == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "world_id is required")
			return
		}

		limit := 5000
		if v := strings.TrimSpace(q.Get("limit")); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid limit")
				return
			}
			if n <= 0 {
				writeError(w, http.StatusBadRequest, "BAD_REQUEST", "limit must be > 0")
				return
			}
			limit = n
		}

		events, err := es.Query(store.Query{WorldID: worldID, SinceTs: 0, Limit: limit})
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		sort.Slice(events, func(i, j int) bool {
			if events[i].Ts != events[j].Ts {
				return events[i].Ts < events[j].Ts
			}
			return events[i].EventID < events[j].EventID
		})

		w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
		// Export format versioning (backward compatible: we only ADD fields to each event JSON object).
		w.Header().Set("X-LW-Export-Schema-Version", "1")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		for _, e := range events {
			// Each Encode writes exactly one JSON object + newline (NDJSON).
			_ = enc.Encode(struct {
				spec.Event
				ExportSchemaVersion int `json:"export_schema_version"`
			}{
				Event:               e,
				ExportSchemaVersion: 1,
			})
		}
	})
}

func buildReplayBeats(worldID string, target spec.Event, prev, next *spec.Event, es store.EventStore, sp *spectator.Projection, sm *sim.Engine) []map[string]any {
	beat1 := target.Narrative

	// --- Shock scripted beats (v0.3+): stabilize highlight structure during shock lifecycle.
	type lifecycle struct {
		warning  *spec.Event
		started  *spec.Event
		betrayal *spec.Event
		ended    *spec.Event
	}
	shockKeyFromEvent := func(e spec.Event) (string, bool) {
		if e.Meta == nil {
			return "", false
		}
		v, ok := e.Meta["shock_key"]
		if !ok || v == nil {
			return "", false
		}
		s, ok := v.(string)
		if !ok {
			return "", false
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return "", false
		}
		return s, true
	}
	resolveShockKeyFromTrace := func(e spec.Event) (string, bool) {
		for _, tl := range e.Trace {
			if tl.CauseEventID == "" {
				continue
			}
			if ce, ok, _ := es.GetByID(worldID, tl.CauseEventID); ok {
				if sk, ok := shockKeyFromEvent(ce); ok {
					return sk, true
				}
			}
		}
		return "", false
	}
	findShockLifecycle := func(sk string, targetTs int64) lifecycle {
		events, err := es.Query(store.Query{WorldID: worldID, SinceTs: 0, Limit: 2000})
		if err != nil {
			return lifecycle{}
		}
		// Deterministic scan order.
		sort.Slice(events, func(i, j int) bool {
			if events[i].Ts != events[j].Ts {
				return events[i].Ts < events[j].Ts
			}
			return events[i].EventID < events[j].EventID
		})
		abs64 := func(x int64) int64 {
			if x < 0 {
				return -x
			}
			return x
		}
		best := func(cur *spec.Event, cand spec.Event) *spec.Event {
			if cur == nil {
				tmp := cand
				return &tmp
			}
			if abs64(cand.Ts-targetTs) < abs64(cur.Ts-targetTs) {
				tmp := cand
				return &tmp
			}
			return cur
		}
		var lc lifecycle
		for _, e := range events {
			k, ok := shockKeyFromEvent(e)
			if !ok || k != sk {
				continue
			}
			switch e.Type {
			case "shock_warning":
				lc.warning = best(lc.warning, e)
			case "shock_started":
				lc.started = best(lc.started, e)
			case "betrayal":
				lc.betrayal = best(lc.betrayal, e)
			case "shock_ended":
				lc.ended = best(lc.ended, e)
			}
		}
		return lc
	}
	prefixIfMissing := func(prefix, s string) string {
		s = strings.TrimSpace(s)
		if s == "" {
			return prefix
		}
		if strings.HasPrefix(s, prefix) {
			return s
		}
		return prefix + s
	}
	enableShockScript := target.Type == "shock_warning" || target.Type == "shock_started" || target.Type == "shock_ended"
	shockKey := ""
	if enableShockScript {
		if sk, ok := shockKeyFromEvent(target); ok {
			shockKey = sk
		}
		if shockKey == "" {
			if sk, ok := resolveShockKeyFromTrace(target); ok {
				shockKey = sk
			}
		}
	}

	// Build documentary-like beats from trace, with graceful fallback to neighbor events.
	// Target: 3~6 beats in total (including opener and ending).
	type step struct {
		prefix string
		note   string
	}
	steps := []step{}

	// Prefer trace-based narration (butterfly effect explanation).
	for i, tl := range target.Trace {
		note := strings.TrimSpace(tl.Note)
		// If this trace points to a concrete cause event, enrich with its narrative.
		if tl.CauseEventID != "" {
			if ce, ok, _ := es.GetByID(worldID, tl.CauseEventID); ok {
				if note == "" {
					note = ce.Narrative
				} else {
					note = note + "（来源：" + ce.Narrative + "）"
				}
			}
		}
		if note == "" {
			continue
		}
		prefix := "进展："
		if i == 0 {
			prefix = "因为："
		} else if i == 1 {
			prefix = "进展："
		} else {
			prefix = "转折："
		}
		steps = append(steps, step{prefix: prefix, note: note})
		if len(steps) >= 4 {
			break
		}
	}
	if len(steps) == 0 && prev != nil {
		steps = append(steps, step{prefix: "铺垫：", note: prev.Narrative})
	}
	if len(steps) == 1 && next != nil {
		steps = append(steps, step{prefix: "余波：", note: next.Narrative})
	}

	beats := make([]map[string]any, 0, 8)
	beats = append(beats, map[string]any{"t": 0, "caption": beat1})

	// Add a compact world-stage line (v0 "解说") based on current sim snapshot.
	if sm != nil {
		if st, ok := sm.GetStatus(worldID); ok {
			// Use projection to add recent-event hook to the summary (state + recent).
			recent := []string{}
			// Prefer trace causes (more meaningful than "行动完成" boilerplate).
			recent = pickRecentFromTrace(es, worldID, target, 2)
			if len(recent) == 0 && sp != nil {
				if home, err := sp.Home(worldID, 10); err == nil {
					recent = pickRecentNarratives(home, 2)
				}
			}
			ws := deriveWorldSummary(st, recent)

			// In shock-scripted mode, keep only stage + 近期 to avoid clutter and structure drift.
			if enableShockScript && shockKey != "" {
				beats = append(beats, map[string]any{"t": 3, "caption": "世界阶段：" + ws.Stage})
				for _, b := range ws.Summary {
					if strings.HasPrefix(b, "近期：") {
						beats = append(beats, map[string]any{"t": 4, "caption": b})
						break
					}
				}
			} else {
				beats = append(beats, map[string]any{"t": 2, "caption": "世界阶段：" + ws.Stage})
				// Add the "近期" bullet as a separate beat (keeps structure stable).
				for _, b := range ws.Summary {
					if strings.HasPrefix(b, "近期：") {
						beats = append(beats, map[string]any{"t": 4, "caption": b})
						break
					}
				}
				// Add one "风险/建议" bullet as a separate beat.
				for _, b := range ws.Summary {
					if strings.HasPrefix(b, "风险：") || strings.HasPrefix(b, "建议：") {
						beats = append(beats, map[string]any{"t": 6, "caption": b})
						break
					}
				}
				// Add one "看点" bullet as a separate beat.
				for _, b := range ws.Summary {
					if strings.HasPrefix(b, "看点：") {
						beats = append(beats, map[string]any{"t": 7, "caption": b})
						break
					}
				}
			}
		}
	}

	// If shockKey is present, emit fixed lifecycle beats to stabilize replay.
	if enableShockScript && shockKey != "" {
		lc := findShockLifecycle(shockKey, target.Ts)
		if lc.warning != nil {
			beats = append(beats, map[string]any{"t": 2, "caption": prefixIfMissing("冲击预警：", lc.warning.Narrative)})
		}
		if lc.started != nil {
			beats = append(beats, map[string]any{"t": 6, "caption": prefixIfMissing("冲击开始：", lc.started.Narrative)})
		}
		if lc.betrayal != nil {
			beats = append(beats, map[string]any{"t": 12, "caption": prefixIfMissing("关系翻转：", lc.betrayal.Narrative)})
		}
		if lc.ended != nil {
			beats = append(beats, map[string]any{"t": 18, "caption": prefixIfMissing("冲击结束：", lc.ended.Narrative)})
		}
		beats = append(beats, map[string]any{"t": 28, "caption": "下一步：关注冲击/背叛/迁徙窗口"})

		sort.SliceStable(beats, func(i, j int) bool {
			ti, _ := beats[i]["t"].(int)
			tj, _ := beats[j]["t"].(int)
			if ti != tj {
				return ti < tj
			}
			ci, _ := beats[i]["caption"].(string)
			cj, _ := beats[j]["caption"].(string)
			return ci < cj
		})
		return beats
	}

	// Spread steps across the middle timeline.
	baseT := 8
	stepGap := 8
	if len(steps) >= 3 {
		stepGap = 6
	}
	for i, st := range steps {
		beats = append(beats, map[string]any{
			"t":       baseT + i*stepGap,
			"caption": st.prefix + st.note,
		})
	}

	// Add a concrete aftermath line for relationship flips (v0).
	// This makes replays more "documentary" instead of generic.
	if target.Type == "betrayal" && len(target.Actors) >= 2 {
		a := target.Actors[0]
		b := target.Actors[1]
		aftermath := "余波：" + a + " 与 " + b + " 翻脸"
		if sp != nil {
			if page, err := sp.Entity(worldID, a, 1); err == nil {
				for _, rr := range page.RelationReasons {
					if rr.To == b && rr.Type == "enemy" && rr.EventID != "" {
						note := rr.Note
						if strings.TrimSpace(note) == "" {
							note = rr.EventID
						}
						aftermath = "余波：" + a + " 与 " + b + " 翻脸（" + note + "）"
						break
					}
				}
			}
		}
		tAfter := baseT + len(steps)*stepGap
		if tAfter > 22 {
			tAfter = 22
		}
		beats = append(beats, map[string]any{
			"t":       tAfter,
			"caption": aftermath,
		})
	}

	beats = append(beats, map[string]any{"t": 28, "caption": "下一步：关注冲击/背叛/迁徙窗口"})

	// Ensure output is stable and easy for clients: sort by t asc, then caption asc.
	sort.SliceStable(beats, func(i, j int) bool {
		ti, _ := beats[i]["t"].(int)
		tj, _ := beats[j]["t"].(int)
		if ti != tj {
			return ti < tj
		}
		ci, _ := beats[i]["caption"].(string)
		cj, _ := beats[j]["caption"].(string)
		return ci < cj
	})

	return beats
}
