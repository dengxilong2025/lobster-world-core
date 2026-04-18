package gateway

import (
	"net/http"
	"strings"

	"lobster-world-core/internal/projections/spectator"
	"lobster-world-core/internal/sim"
)

func registerSpectatorRoutes(mux *http.ServeMux, sp *spectator.Projection, sm *sim.Engine) {
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

		// Derive a minimal world stage/summary from the sim snapshot (v0 "解说" layer).
		var world any = nil
		if st, ok := sm.GetStatus(worldID); ok {
			world = deriveWorldSummary(st, pickRecentNarratives(home, 2))
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
			"ok":         true,
			"world_id":   worldID,
			"world":      world,
			"headline":   headline,
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
			"ok":              true,
			"world_id":        worldID,
			"entity_id":       entityID,
			"relations":       page.Relations,
			"relation_reasons": page.RelationReasons,
			"recent_events":   page.RecentEvents,
			"why_strong":      page.WhyStrong,
			"next_risk":       page.NextRisk,
		})
	})
}
