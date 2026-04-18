package sim

import (
	"fmt"
	"sync"
	"time"

	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
)

// Engine manages per-world simulation loops (one goroutine per world_id).
//
// Concurrency model (v0):
// - Within a world_id: single goroutine processes intents and ticks -> deterministic ordering.
// - Across world_id: worlds run concurrently -> horizontal scalability.
type Engine struct {
	mu sync.Mutex

	es  store.EventStore
	hub *stream.Hub

	tickInterval time.Duration
	shock        *ShockConfig
	seed         int64

	worlds map[string]*world
	stopped bool
}

type Options struct {
	EventStore    store.EventStore
	Hub           *stream.Hub
	TickInterval  time.Duration // default 5s (product choice B)
	Shock         *ShockConfig
	Seed          int64
}

func New(opts Options) *Engine {
	ti := opts.TickInterval
	if ti <= 0 {
		ti = 5 * time.Second
	}
	return &Engine{
		es:           opts.EventStore,
		hub:          opts.Hub,
		tickInterval: ti,
		shock:        opts.Shock,
		seed:         opts.Seed,
		worlds:       map[string]*world{},
		stopped:      false,
	}
}

func (e *Engine) EnsureWorld(worldID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.stopped {
		return
	}
	if _, ok := e.worlds[worldID]; ok {
		return
	}
	w := newWorld(worldID, e.tickInterval, e.es, e.hub)
	w.setSeed(deriveWorldSeed(e.seed, worldID))
	if e.shock != nil {
		w.setShockScheduler(newShockScheduler(*e.shock, w.seed))
	}
	e.worlds[worldID] = w
	w.start()
}

func (e *Engine) SubmitIntent(worldID string, in Intent) (intentID string, err error) {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return "", fmt.Errorf("engine stopped")
	}
	e.mu.Unlock()
	e.EnsureWorld(worldID)

	e.mu.Lock()
	w := e.worlds[worldID]
	e.mu.Unlock()

	if w == nil {
		return "", fmt.Errorf("world not available")
	}
	id, ack := w.submitIntent(in)
	select {
	case aerr := <-ack:
		if aerr != nil {
			return "", aerr
		}
		return id, nil
	case <-time.After(2 * time.Second):
		return "", fmt.Errorf("timeout waiting for intent acceptance")
	}
}

func (e *Engine) GetStatus(worldID string) (Status, bool) {
	e.mu.Lock()
	w := e.worlds[worldID]
	e.mu.Unlock()
	if w == nil {
		return Status{}, false
	}
	return w.status(), true
}

// Stop terminates all world goroutines and prevents new worlds from being created.
// It is safe to call multiple times.
func (e *Engine) Stop() {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return
	}
	e.stopped = true
	worlds := make([]*world, 0, len(e.worlds))
	for _, w := range e.worlds {
		worlds = append(worlds, w)
	}
	e.mu.Unlock()

	for _, w := range worlds {
		w.stop()
	}
}
