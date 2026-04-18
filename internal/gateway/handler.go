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

	registerHealthRoutes(mux)
	registerAuthRoutes(mux, a, authLimiter)
	registerEventRoutes(mux, es, hub)
	registerIntentRoutes(mux, sm)
	registerAdoptionRoutes(mux, a, ad, es, hub)
	registerSpectatorRoutes(mux, sp, sm)
	registerReplayRoutes(mux, es, sp)

	return mux
}
