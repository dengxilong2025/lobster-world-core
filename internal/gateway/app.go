package gateway

import (
	"net/http"

	"lobster-world-core/internal/auth"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
)

// App bundles server dependencies so tests and main can share the same wiring.
type App struct {
	Handler http.Handler

	Auth       *auth.Service
	EventStore store.EventStore
	Hub        *stream.Hub
}

// NewApp constructs the v0 application with in-memory implementations.
func NewApp() *App {
	a := &App{
		Auth:       auth.NewService(auth.Options{}),
		EventStore: store.NewInMemoryEventStore(),
		Hub:        stream.NewHub(),
	}
	a.Handler = NewHandler(Options{
		Auth:       a.Auth,
		EventStore: a.EventStore,
		Hub:        a.Hub,
	})
	return a
}

