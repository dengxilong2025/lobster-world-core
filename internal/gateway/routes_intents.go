package gateway

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"lobster-world-core/internal/sim"
)

func registerIntentRoutes(mux *http.ServeMux, sm *sim.Engine, mt *Metrics) {
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

		start := time.Now()
		intentID, err := sm.SubmitIntent(worldID, sim.Intent{
			Goal:        req.Goal,
			Constraints: req.Constraints,
			Horizon:     req.Horizon,
			Risk:        req.Risk,
			Notes:       req.Notes,
		})
		if err != nil {
			if errors.Is(err, sim.ErrBusy) {
				if mt != nil {
					mt.IncBusy()
				}
				writeError(w, http.StatusServiceUnavailable, "BUSY", "world is busy")
				return
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL", "failed to persist intent")
			return
		}

		if mt != nil {
			ms := time.Since(start).Milliseconds()
			if ms == 0 {
				ms = 1
			}
			mt.AddIntentAcceptWaitMs(ms)
			mt.IncIntentAcceptWaitCount()
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"world_id":  worldID,
			"intent_id": intentID,
			"accepted":  true,
		})
	})
}
