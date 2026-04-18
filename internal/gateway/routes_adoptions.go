package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"lobster-world-core/internal/adoption"
	"lobster-world-core/internal/auth"
	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
)

func registerAdoptionRoutes(mux *http.ServeMux, a *auth.Service, ad *adoption.Service, es store.EventStore, hub *stream.Hub) {
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
}

