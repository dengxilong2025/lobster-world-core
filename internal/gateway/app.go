package gateway

import (
	"net/http"
	"runtime/debug"
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
	Metrics    *Metrics

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

	// IntentAcceptTimeout bounds how long SubmitIntent waits for durable acceptance.
	// If 0, sim defaults are used (currently 2s).
	IntentAcceptTimeout time.Duration

	// IntentChannelCap controls the per-world submission channel capacity.
	// nil means default (256). 0 means unbuffered (useful for deterministic tests).
	IntentChannelCap *int

	// TrustedProxyCIDRs configures reverse proxies that are allowed to set X-Forwarded-For.
	// If empty, only loopback proxies are trusted (safe default).
	TrustedProxyCIDRs []string

	// ReadBuildInfo allows tests to simulate environments where runtime/debug.ReadBuildInfo returns
	// no buildvcs info (e.g. Docker runtime without VCS metadata).
	// If nil, runtime/debug.ReadBuildInfo is used.
	ReadBuildInfo func() (*debug.BuildInfo, bool)

	// GitHubCommitResolver allows tests to inject a resolver (e.g. httptest-backed base URL).
	// If nil, NewHandler will create a default resolver.
	GitHubCommitResolver GitHubCommitResolver
}

func NewAppWithOptions(opts AppOptions) *App {
	mt := NewMetrics()
	es := wrapEventStoreWithMetrics(store.NewInMemoryEventStore(), mt)

	a := &App{
		Auth:       auth.NewService(auth.Options{}),
		EventStore: es,
		Hub:        stream.NewHub(),
		Adoption:   adoption.NewService(adoption.Options{}),
		Metrics:    mt,
	}
	a.Spectator = spectator.New(spectator.Options{EventStore: a.EventStore})

	// Best-effort prewarm for the default world so the first spectator request doesn't pay the
	// cold-start rebuild cost (useful once EventStore persists across restarts).
	go func() { _ = a.Spectator.EnsureLoaded(DefaultWorldID) }()

	a.Sim = sim.New(sim.Options{
		EventStore:   a.EventStore,
		Hub:          a.Hub,
		TickInterval: opts.TickInterval,
		IntentAcceptTimeout: opts.IntentAcceptTimeout,
		Shock:        opts.Shock,
		Seed:         opts.Seed,
		MaxIntentQueue: opts.MaxIntentQueue,
		IntentChannelCap: opts.IntentChannelCap,
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
		Metrics:    a.Metrics,
		ReadBuildInfo:        opts.ReadBuildInfo,
		GitHubCommitResolver: opts.GitHubCommitResolver,
		TrustedProxyCIDRs: opts.TrustedProxyCIDRs,
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
