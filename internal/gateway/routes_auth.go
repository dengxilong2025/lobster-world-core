package gateway

import (
	"encoding/json"
	"net"
	"net/http"

	"lobster-world-core/internal/auth"
)

func registerAuthRoutes(mux *http.ServeMux, a *auth.Service, limiter *ipRateLimiter, trustedProxies []*net.IPNet) {
	mux.Handle("POST /api/v0/auth/challenge", rateLimitWithTrusted(limiter, trustedProxies, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	mux.Handle("POST /api/v0/auth/prove", rateLimitWithTrusted(limiter, trustedProxies, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			"ok":             true,
			"lobster_id":     lobsterID,
			"lobster_pubkey": pubkey,
		})
	})
}
