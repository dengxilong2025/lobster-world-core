package sim

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
)

type world struct {
	worldID string

	es  store.EventStore
	hub *stream.Hub

	tickInterval time.Duration

	mu        sync.Mutex
	tick      int64
	eventSeq  int64
	intentSeq int64
	queue     []queuedIntent

	intentCh chan queuedIntent
	stopCh   chan struct{}

	state WorldState

	shocks *shockScheduler
}

type queuedIntent struct {
	ID     string
	Intent Intent
}

func newWorld(worldID string, tickInterval time.Duration, es store.EventStore, hub *stream.Hub) *world {
	if tickInterval <= 0 {
		tickInterval = 5 * time.Second
	}
	return &world{
		worldID:      worldID,
		es:           es,
		hub:          hub,
		tickInterval: tickInterval,
		intentCh:     make(chan queuedIntent, 256),
		stopCh:       make(chan struct{}),
		state: WorldState{
			// Start from a small but non-zero baseline so spectator views feel alive.
			Food:       100,
			Population: 100,
			Order:      50,
			Trust:      50,
			Knowledge:  0,
			Conflict:   0,
		},
	}
}

func (w *world) setShockScheduler(s *shockScheduler) {
	w.mu.Lock()
	w.shocks = s
	w.mu.Unlock()
}

func (w *world) start() {
	go w.loop()
}

func (w *world) submitIntent(in Intent) string {
	w.mu.Lock()
	w.intentSeq++
	id := fmt.Sprintf("int_%s_%d", sanitize(w.worldID), w.intentSeq)
	w.mu.Unlock()

	w.intentCh <- queuedIntent{ID: id, Intent: in}
	return id
}

func (w *world) loop() {
	t := time.NewTicker(w.tickInterval)
	defer t.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case qi := <-w.intentCh:
			w.handleIntent(qi)
		case <-t.C:
			w.step()
		}
	}
}

func (w *world) handleIntent(qi queuedIntent) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Enqueue for future execution.
	w.queue = append(w.queue, qi)

	// Emit intent_accepted immediately at current tick (tick=0 at world start is allowed).
	ev := w.newEventLocked("intent_accepted", []string{qi.ID}, fmt.Sprintf("意图接受：%s", qi.Intent.Goal))
	w.appendAndPublish(ev)
}

func (w *world) step() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.tick++

	// Shock scheduler runs at tick boundaries.
	if w.shocks != nil {
		evs := w.shocks.step(w.worldID, w.tick)
		now := time.Now().Unix()
		for _, ev := range evs {
			ev.Ts = now
			w.appendAndPublish(ev)
		}
	}

	if len(w.queue) == 0 {
		return
	}

	// Execute at most one intent per tick (v0 throttle; easy to reason about).
	qi := w.queue[0]
	w.queue = w.queue[1:]

	started := w.newEventLocked("action_started", []string{qi.ID}, "行动开始：执行意图")
	started.Tick = w.tick
	w.appendAndPublish(started)

	done := w.newEventLocked("action_completed", []string{qi.ID}, "行动完成：意图执行完毕")
	done.Tick = w.tick
	// Minimal delta for now (will be replaced by real world model).
	done.Delta = map[string]any{"knowledge": 1}
	w.state.ApplyDelta(done.Delta)
	w.appendAndPublish(done)
}

func (w *world) newEventLocked(typ string, actors []string, narrative string) spec.Event {
	w.eventSeq++
	return spec.Event{
		SchemaVersion: 1,
		EventID:       fmt.Sprintf("evt_%s_%d_%d", sanitize(w.worldID), w.tick, w.eventSeq),
		Ts:            time.Now().Unix(),
		WorldID:       w.worldID,
		Scope:         "world",
		Type:          typ,
		Actors:        actors,
		Narrative:     narrative,
		Tick:          w.tick,
	}
}

func (w *world) appendAndPublish(ev spec.Event) {
	if w.es != nil {
		_ = w.es.Append(ev)
	}
	if w.hub != nil {
		w.hub.Publish(ev)
	}
}

type Status struct {
	WorldID string
	Tick    int64
	State   WorldState
}

func (w *world) status() Status {
	w.mu.Lock()
	defer w.mu.Unlock()

	return Status{
		WorldID: w.worldID,
		Tick:    w.tick,
		State:   w.state,
	}
}

var sanitizeRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitize(s string) string {
	if s == "" {
		return "world"
	}
	return sanitizeRe.ReplaceAllString(s, "_")
}
