package spectator

import (
	"sync"
	"testing"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
)

type countingStore struct {
	mu    sync.Mutex
	n     int
	delay time.Duration
	evs   []spec.Event
}

func (c *countingStore) Append(e spec.Event) error { return nil }
func (c *countingStore) GetByID(worldID, eventID string) (spec.Event, bool, error) {
	return spec.Event{}, false, nil
}
func (c *countingStore) Query(q store.Query) ([]spec.Event, error) {
	c.mu.Lock()
	c.n++
	delay := c.delay
	evs := append([]spec.Event{}, c.evs...)
	c.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	// evs is assumed already ts asc, which matches store.Query contract.
	return evs, nil
}
func (c *countingStore) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func TestProjection_EnsureLoaded_DoesNotRequeryWhenCached(t *testing.T) {
	t.Parallel()

	es := &countingStore{
		evs: []spec.Event{
			{SchemaVersion: 1, EventID: "evt_1", Ts: 1, Tick: 1, WorldID: "w1", Scope: "world", Type: "x", Actors: []string{"a"}, Narrative: "n1"},
		},
	}
	p := New(Options{EventStore: es, Limit: 50})

	if err := p.EnsureLoaded("w1"); err != nil {
		t.Fatalf("EnsureLoaded: %v", err)
	}
	if err := p.EnsureLoaded("w1"); err != nil {
		t.Fatalf("EnsureLoaded(2): %v", err)
	}

	// EnsureLoaded may call Query twice during the initial rebuild (events + raw for relations),
	// but subsequent calls should be a no-op. We assert a single query in the current impl.
	if got := es.Count(); got != 1 {
		t.Fatalf("expected Query count=1, got %d", got)
	}
}

func TestProjection_EnsureLoaded_SingleflightPerWorld(t *testing.T) {
	t.Parallel()

	es := &countingStore{
		delay: 50 * time.Millisecond,
		evs: []spec.Event{
			{SchemaVersion: 1, EventID: "evt_1", Ts: 1, Tick: 1, WorldID: "w1", Scope: "world", Type: "x", Actors: []string{"a"}, Narrative: "n1"},
		},
	}
	p := New(Options{EventStore: es, Limit: 50})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = p.EnsureLoaded("w1") }()
	go func() { defer wg.Done(); _ = p.EnsureLoaded("w1") }()
	wg.Wait()

	if got := es.Count(); got != 1 {
		t.Fatalf("expected Query count=1 (singleflight), got %d", got)
	}
}
