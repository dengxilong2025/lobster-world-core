package gateway

import "net/http"

// NewHandler returns the root HTTP handler for the service.
//
// v0 (P0) only exposes /healthz.
// Later tasks will add auth, events, intents, adoptions, etc.
func NewHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return mux
}

