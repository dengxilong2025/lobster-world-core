package gateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"lobster-world-core/internal/auth"
)

// NewHandler returns the root HTTP handler for the service.
//
// v0 (P0) only exposes /healthz.
// Later tasks will add auth, events, intents, adoptions, etc.
func NewHandler() http.Handler {
	a := auth.NewService(auth.Options{})

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// API v0
	mux.HandleFunc("POST /api/v0/auth/challenge", func(w http.ResponseWriter, r *http.Request) {
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
	})

	mux.HandleFunc("POST /api/v0/auth/prove", func(w http.ResponseWriter, r *http.Request) {
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
	})

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
