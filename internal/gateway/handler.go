package gateway

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"lobster-world-core/internal/auth"
	"lobster-world-core/internal/adoption"
	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
	"lobster-world-core/internal/projections/spectator"
	"lobster-world-core/internal/sim"
)

type Options struct {
	Auth       *auth.Service
	EventStore store.EventStore
	Hub        *stream.Hub
	Adoption   *adoption.Service
	Spectator  *spectator.Projection
	Sim        *sim.Engine
}

// NewHandler returns the root HTTP handler for the service.
// This is the main wiring point for HTTP endpoints.
func NewHandler(opts Options) http.Handler {
	a := opts.Auth
	if a == nil {
		a = auth.NewService(auth.Options{})
	}
	es := opts.EventStore
	if es == nil {
		es = store.NewInMemoryEventStore()
	}
	hub := opts.Hub
	if hub == nil {
		hub = stream.NewHub()
	}
	ad := opts.Adoption
	if ad == nil {
		ad = adoption.NewService(adoption.Options{})
	}
	sp := opts.Spectator
	if sp == nil {
		sp = spectator.New(spectator.Options{EventStore: es})
	}
	sm := opts.Sim
	if sm == nil {
		sm = sim.New(sim.Options{EventStore: es, Hub: hub})
	}

	mux := http.NewServeMux()

	// v0 abuse protection: simple IP-based rate limit for auth endpoints.
	// Default policy: allow short burst then throttle.
	authLimiter := newIPRateLimiter(2, 2, 10*time.Minute) // 2 req/sec with burst 2 per IP

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// API v0
	mux.Handle("POST /api/v0/auth/challenge", rateLimit(authLimiter, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			LobsterPubkey string `json:"lobster_pubkey"`
			ClientTs      int64  `json:"client_ts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
			return
		}
		ch, ttl, err := a.CreateChallenge(req.LobsterPubkey)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"challenge": ch,
			"ttl_sec":   ttl,
		})
	})))

	mux.Handle("POST /api/v0/auth/prove", rateLimit(authLimiter, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			LobsterPubkey string `json:"lobster_pubkey"`
			Challenge     string `json:"challenge"`
			Sig           string `json:"sig"`
			ClientTs      int64  `json:"client_ts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
			return
		}
		token, exp, lobsterID, err := a.Prove(req.LobsterPubkey, req.Challenge, req.Sig)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"session_token": token,
			"expires_at":    exp,
			"lobster_id":    lobsterID,
		})
	})))

	mux.HandleFunc("GET /api/v0/me", func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token")
			return
		}
		lobsterID, pubkey, ok := a.GetSession(token)
		if !ok {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid session")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":            true,
			"lobster_id":    lobsterID,
			"lobster_pubkey": pubkey,
		})
	})

	mux.HandleFunc("GET /api/v0/events", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		worldID := q.Get("world_id")
		sinceTs := parseInt64(q.Get("since_ts"))
		limit := parseInt(q.Get("limit"))
		entityID := q.Get("entity_id")

		events, err := es.Query(store.Query{
			WorldID:  worldID,
			SinceTs:  sinceTs,
			Limit:    limit,
			EntityID: entityID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"events": events,
		})
	})

	// SSE event stream. Transport is decoupled from the event object.
	mux.HandleFunc("GET /api/v0/events/stream", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		worldID := q.Get("world_id")
		if worldID == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "world_id is required")
			return
		}
		sinceTs := parseInt64(q.Get("since_ts"))

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "streaming unsupported")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Initial comment to establish stream.
		_, _ = w.Write([]byte(":ok\n\n"))
		flusher.Flush()

		ch, unsub := hub.Subscribe(256)
		defer unsub()

		bw := bufio.NewWriter(w)

		// Replay missed events first (best-effort). This is critical because hub may drop under backpressure.
		if sinceTs > 0 {
			missed, err := es.Query(store.Query{WorldID: worldID, SinceTs: sinceTs, Limit: 500})
			if err == nil {
				for _, e := range missed {
					b, _ := json.Marshal(e)
					_, _ = bw.WriteString("event: message\n")
					_, _ = bw.WriteString("data: ")
					_, _ = bw.Write(b)
					_, _ = bw.WriteString("\n\n")
				}
				_ = bw.Flush()
				flusher.Flush()
			}
		}

		for {
			select {
			case <-r.Context().Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				if e.WorldID != worldID {
					continue
				}
				b, _ := json.Marshal(e)
				_, _ = bw.WriteString("event: message\n")
				_, _ = bw.WriteString("data: ")
				_, _ = bw.Write(b)
				_, _ = bw.WriteString("\n\n")
				_ = bw.Flush()
				flusher.Flush()
			}
		}
	})

	// Minimal intent endpoint (v0 placeholder executor).
	mux.HandleFunc("POST /api/v0/intents", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			WorldID     string   `json:"world_id"`
			Goal        string   `json:"goal"`
			Constraints []string `json:"constraints"`
			Horizon     string   `json:"horizon"`
			Risk        string   `json:"risk"`
			Notes       string   `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
			return
		}
		if strings.TrimSpace(req.Goal) == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "goal is required")
			return
		}

		worldID := req.WorldID
		if strings.TrimSpace(worldID) == "" {
			worldID = DefaultWorldID
		}

		intentID, err := sm.SubmitIntent(worldID, sim.Intent{
			Goal:        req.Goal,
			Constraints: req.Constraints,
			Horizon:     req.Horizon,
			Risk:        req.Risk,
			Notes:       req.Notes,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "failed to persist intent")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"world_id":  worldID,
			"intent_id": intentID,
			"accepted":  true,
		})
	})

	// Adoption/binding endpoints (default unbind).
	mux.HandleFunc("POST /api/v0/adoptions/confirm", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			HumanPubkey string `json:"human_pubkey"`
			LobsterID   string `json:"lobster_id"`
			Sig         string `json:"sig"`
			ClientTs    int64  `json:"client_ts"`
			Nonce       string `json:"nonce"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
			return
		}
		if req.HumanPubkey == "" || req.LobsterID == "" || req.Sig == "" || req.Nonce == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing required fields")
			return
		}
		humanID, err := ad.ConfirmByHumanSig(req.HumanPubkey, req.LobsterID, req.Sig, req.ClientTs, req.Nonce)
		if err != nil {
			code := "BAD_REQUEST"
			if strings.Contains(err.Error(), "cooldown") {
				code = "COOLDOWN"
			}
			writeError(w, http.StatusBadRequest, code, err.Error())
			return
		}

		ev := spec.Event{
			SchemaVersion: 1,
			EventID:       "evt_" + randID(),
			Ts:            time.Now().Unix(),
			WorldID:       DefaultWorldID,
			Scope:         "world",
			Type:          "adoption_confirmed",
			Actors:        []string{humanID, req.LobsterID},
			Narrative:     fmt.Sprintf("领养成立：%s 绑定 %s", humanID, req.LobsterID),
		}
		if err := es.Append(ev); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "failed to persist event")
			return
		}
		hub.Publish(ev)

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"event_id": ev.EventID,
			"human_id": humanID,
		})
	})

	mux.HandleFunc("POST /api/v0/adoptions/revoke", func(w http.ResponseWriter, r *http.Request) {
		// Option A: lobster side via bearer token.
		if token := bearerToken(r.Header.Get("Authorization")); token != "" {
			lobsterID, _, ok := a.GetSession(token)
			if !ok {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid session")
				return
			}
			humanID, cooldownSec, err := ad.RevokeByLobster(lobsterID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
				return
			}
			ev := spec.Event{
				SchemaVersion: 1,
				EventID:       "evt_" + randID(),
				Ts:            time.Now().Unix(),
				WorldID:       DefaultWorldID,
				Scope:         "world",
				Type:          "adoption_revoked",
				Actors:        []string{humanID, lobsterID},
				Narrative:     fmt.Sprintf("解绑完成：%s 与 %s 解除绑定", humanID, lobsterID),
			}
			if err := es.Append(ev); err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL", "failed to persist event")
				return
			}
			hub.Publish(ev)
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":           true,
				"event_id":     ev.EventID,
				"cooldown_sec": cooldownSec,
			})
			return
		}

		// Option B: human side via signature.
		var req struct {
			HumanPubkey string `json:"human_pubkey"`
			LobsterID   string `json:"lobster_id"`
			Sig         string `json:"sig"`
			ClientTs    int64  `json:"client_ts"`
			Nonce       string `json:"nonce"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
			return
		}
		if req.HumanPubkey == "" || req.LobsterID == "" || req.Sig == "" || req.Nonce == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing required fields")
			return
		}
		humanID, cooldownSec, err := ad.RevokeByHumanSig(req.HumanPubkey, req.LobsterID, req.Sig, req.ClientTs, req.Nonce)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		ev := spec.Event{
			SchemaVersion: 1,
			EventID:       "evt_" + randID(),
			Ts:            time.Now().Unix(),
			WorldID:       DefaultWorldID,
			Scope:         "world",
			Type:          "adoption_revoked",
			Actors:        []string{humanID, req.LobsterID},
			Narrative:     fmt.Sprintf("解绑完成：%s 与 %s 解除绑定", humanID, req.LobsterID),
		}
		if err := es.Append(ev); err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "failed to persist event")
			return
		}
		hub.Publish(ev)
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"event_id":     ev.EventID,
			"cooldown_sec": cooldownSec,
		})
	})

	// Spectator home (v0). This is a projection view built from the event log.
	mux.HandleFunc("GET /api/v0/spectator/home", func(w http.ResponseWriter, r *http.Request) {
		worldID := r.URL.Query().Get("world_id")
		if worldID == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "world_id is required")
			return
		}
		home, err := sp.Home(worldID, 10)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}

		headline := map[string]any{}
		if home.Headline != nil {
			headline = map[string]any{
				"event_id":   home.Headline.EventID,
				"type":       home.Headline.Type,
				"title":      home.Headline.Narrative,
				"narrative":  home.Headline.Narrative,
				"replay_id":  "rp_" + home.Headline.EventID,
			}
		}

		hot := make([]map[string]any, 0, len(home.HotEvents))
		for _, e := range home.HotEvents {
			hot = append(hot, map[string]any{
				"event_id":  e.EventID,
				"type":      e.Type,
				"title":     e.Narrative,
				"narrative": e.Narrative,
				"replay_id": "rp_" + e.EventID,
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"world_id":  worldID,
			"headline":  headline,
			"hot_events": hot,
		})
	})

	// World status (v0). For now it's a direct in-memory snapshot from the sim engine.
	// Later it can be moved into a projection/read-model store.
	mux.HandleFunc("GET /api/v0/world/status", func(w http.ResponseWriter, r *http.Request) {
		worldID := r.URL.Query().Get("world_id")
		if strings.TrimSpace(worldID) == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "world_id is required")
			return
		}
		st, ok := sm.GetStatus(worldID)
		if !ok {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "world not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"world_id": st.WorldID,
			"tick":     st.Tick,
			"state": map[string]any{
				"food":       st.State.Food,
				"population": st.State.Population,
				"order":      st.State.Order,
				"trust":      st.State.Trust,
				"knowledge":  st.State.Knowledge,
				"conflict":   st.State.Conflict,
			},
		})
	})

	mux.HandleFunc("GET /api/v0/spectator/entity", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		worldID := q.Get("world_id")
		entityID := q.Get("entity_id")
		if worldID == "" || entityID == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "world_id and entity_id are required")
			return
		}
		page, err := sp.Entity(worldID, entityID, 20)
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":           true,
			"world_id":     worldID,
			"entity_id":    entityID,
			"relations":    page.Relations,
			"relation_reasons": page.RelationReasons,
			"recent_events": page.RecentEvents,
			"why_strong":   page.WhyStrong,
			"next_risk":    page.NextRisk,
		})
	})

	// Replay highlight (MVP): return a structured "script replay" for 30s.
	mux.HandleFunc("GET /api/v0/replay/highlight", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		worldID := q.Get("world_id")
		eventID := q.Get("event_id")
		if worldID == "" || eventID == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "world_id and event_id are required")
			return
		}
		events, err := es.Query(store.Query{WorldID: worldID, SinceTs: 0, Limit: 1000})
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
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

		beat1 := target.Narrative
		// Build documentary-like beats from trace, with graceful fallback to neighbor events.
		// Target: 3~6 beats in total (including opener and ending).
		type step struct {
			prefix  string
			fallback string
			note    string
		}
		steps := []step{}

		// Prefer trace-based narration (butterfly effect explanation).
		for i, tl := range target.Trace {
			if strings.TrimSpace(tl.Note) == "" {
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
			steps = append(steps, step{prefix: prefix, note: tl.Note})
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

		beats := make([]map[string]any, 0, 6)
		beats = append(beats, map[string]any{"t": 0, "caption": beat1})

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
			tAfter := baseT + len(steps)*stepGap
			if tAfter > 22 {
				tAfter = 22
			}
			beats = append(beats, map[string]any{
				"t":       tAfter,
				"caption": aftermath,
			})
		}

		beats = append(beats, map[string]any{
			"t":       24,
			"caption": "下一步：关注冲击/背叛/迁徙窗口",
		})

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"replay_id": "rp_" + target.EventID,
			"event_id":  target.EventID,
			"duration_sec": 30,
			"beats":     beats,
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
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
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
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		for _, e := range events {
			// Each Encode writes exactly one JSON object + newline (NDJSON).
			_ = enc.Encode(e)
		}
	})

	return mux
}

func bearerToken(h string) string {
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"ok": false,
		"error": map[string]any{
			"code": code,
			"msg":  msg,
		},
	})
}

func randID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func parseInt64(v string) int64 {
	if v == "" {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func parseInt(v string) int {
	if v == "" {
		return 0
	}
	n, _ := strconv.Atoi(v)
	return n
}
