package gateway

import (
	"net/http"
	"time"

	"lobster-world-core/internal/adoption"
	"lobster-world-core/internal/auth"
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

	// TrustedProxyCIDRs configures reverse proxies that are allowed to set X-Forwarded-For.
	// If empty, only loopback proxies are trusted (safe default).
	TrustedProxyCIDRs []string
}

// NewHandler returns the root HTTP handler for the service.
// This is the main wiring point for HTTP endpoints.
func NewHandler(opts Options) http.Handler {
	// Metrics (debug/ops): initialized per handler wiring.
	// Note: we keep a package-level pointer so writeError can tag BUSY without changing all signatures.
	mt := NewMetrics()
	setDefaultMetrics(mt)

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
	trusted, _ := parseTrustedProxyCIDRs(opts.TrustedProxyCIDRs)

	registerHealthRoutes(mux)
	registerAuthRoutes(mux, a, authLimiter, trusted)
	registerEventRoutes(mux, es, hub)
	registerIntentRoutes(mux, sm)
	registerAdoptionRoutes(mux, a, ad, es, hub)
	registerSpectatorRoutes(mux, sp, sm)
	registerReplayRoutes(mux, es, sp, sm)
	registerAssetRoutes(mux)
	registerUIRoutes(mux)
	registerDebugRoutes(mux, sm, opts.TrustedProxyCIDRs, mt)

	// Wrap mux to capture status codes for metrics.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mt.IncRequest()
		srw := &statusCapturingResponseWriter{ResponseWriter: w, status: 200}
		mux.ServeHTTP(srw, r)
		mt.IncStatus(srw.status)
	})
}

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Flush forwards http.Flusher when supported by the underlying writer.
// This is required for SSE endpoints.
func (w *statusCapturingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
