package store

import (
	"fmt"
	"sort"
	"sync"

	"lobster-world-core/internal/events/spec"
)

// Query defines how to fetch events from the EventStore.
// v0 keeps this intentionally small and stable.
type Query struct {
	WorldID  string
	SinceTs  int64
	Limit    int
	EntityID string // reserved for later (entity-scoped queries)
}

// EventStore is an append-only log for canonical event objects.
// Implementations MUST be safe for concurrent use.
type EventStore interface {
	Append(e spec.Event) error
	Query(q Query) ([]spec.Event, error)
}

// InMemoryEventStore is a minimal v0 implementation for local development and tests.
// It is append-only, enforces unique event_id, and supports querying by world_id + since_ts.
type InMemoryEventStore struct {
	mu sync.RWMutex

	byWorld map[string][]spec.Event
	seenID  map[string]struct{}
}

func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		byWorld: map[string][]spec.Event{},
		seenID:  map[string]struct{}{},
	}
}

func (s *InMemoryEventStore) Append(e spec.Event) error {
	if err := e.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.seenID[e.EventID]; ok {
		return fmt.Errorf("duplicate event_id: %s", e.EventID)
	}
	s.seenID[e.EventID] = struct{}{}

	s.byWorld[e.WorldID] = append(s.byWorld[e.WorldID], e)

	// Keep per-world slice sorted by (ts, event_id) so queries are stable.
	sort.Slice(s.byWorld[e.WorldID], func(i, j int) bool {
		a := s.byWorld[e.WorldID][i]
		b := s.byWorld[e.WorldID][j]
		if a.Ts != b.Ts {
			return a.Ts < b.Ts
		}
		return a.EventID < b.EventID
	})

	return nil
}

func (s *InMemoryEventStore) Query(q Query) ([]spec.Event, error) {
	if q.WorldID == "" {
		return nil, fmt.Errorf("world_id is required")
	}
	if q.Limit <= 0 {
		q.Limit = 200
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	events := s.byWorld[q.WorldID]
	if len(events) == 0 {
		return []spec.Event{}, nil
	}

	out := make([]spec.Event, 0, min(q.Limit, len(events)))
	for _, e := range events {
		if q.SinceTs > 0 && e.Ts <= q.SinceTs {
			continue
		}
		// EntityID is reserved for later; ignore in v0.
		out = append(out, e)
		if len(out) >= q.Limit {
			break
		}
	}
	return out, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

