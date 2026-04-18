package gateway

import (
	"net/http"
	"sync"
	"time"

	"lobster-world-core/internal/adoption"
	"lobster-world-core/internal/auth"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
	"lobster-world-core/internal/projections/spectator"
	"lobster-world-core/internal/sim"
)

// App bundles server dependencies so tests and main can share the same wiring.
type App struct {
	Handler http.Handler

	Auth       *auth.Service
	EventStore store.EventStore
	Hub        *stream.Hub
	Adoption   *adoption.Service
	Spectator  *spectator.Projection
	Sim        *sim.Engine

	stopOnce  sync.Once
	hubUnsub  func()
}

// NewApp constructs the v0 application with in-memory implementations.
func NewApp() *App {
	return NewAppWithOptions(AppOptions{})
}

type AppOptions struct {
	// TickInterval controls how fast the sim advances. Default is 5s (product choice B).
	// Tests can override this to be much faster.
	TickInterval time.Duration

	// Shock configures the shock scheduler (P2-M3). If nil, scheduler is off.
	Shock *sim.ShockConfig

	// Seed controls determinism for simulation randomness (P3-M3).
	// If 0, the engine derives a stable default from world_id.
	Seed int64

	// MaxIntentQueue bounds the number of pending intents per world (safety valve).
	MaxIntentQueue int
}

func NewAppWithOptions(opts AppOptions) *App {
	a := &App{
		Auth:       auth.NewService(auth.Options{}),
		EventStore: store.NewInMemoryEventStore(),
		Hub:        stream.NewHub(),
		Adoption:   adoption.NewService(adoption.Options{}),
	}
	a.Spectator = spectator.New(spectator.Options{EventStore: a.EventStore})
	a.Sim = sim.New(sim.Options{
		EventStore:   a.EventStore,
		Hub:          a.Hub,
		TickInterval: opts.TickInterval,
		Shock:        opts.Shock,
		Seed:         opts.Seed,
		MaxIntentQueue: opts.MaxIntentQueue,
	})

	// Keep spectator projection realtime by subscribing to the in-process hub.
	// This prevents "stale snapshot" after the first EnsureLoaded() call.
	ch, unsub := a.Hub.Subscribe(256)
	a.hubUnsub = unsub
	go func() {
		for e := range ch {
			a.Spectator.Apply(e)
		}
	}()

	a.Handler = NewHandler(Options{
		Auth:       a.Auth,
		EventStore: a.EventStore,
		Hub:        a.Hub,
		Adoption:   a.Adoption,
		Spectator:  a.Spectator,
		Sim:        a.Sim,
	})
	return a
}

// Stop gracefully stops background goroutines owned by the app (sim + realtime projection).
// Safe to call multiple times.
func (a *App) Stop() {
	a.stopOnce.Do(func() {
		if a.hubUnsub != nil {
			a.hubUnsub()
		}
		if a.Sim != nil {
			a.Sim.Stop()
		}
	})
}
