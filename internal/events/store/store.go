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
	// GetByID returns the event with the given event_id in the given world.
	// Implementations should be O(1) where possible (e.g. using a secondary index).
	GetByID(worldID, eventID string) (spec.Event, bool, error)
}

// InMemoryEventStore is a minimal v0 implementation for local development and tests.
// It is append-only, enforces unique event_id, and supports querying by world_id + since_ts.
type InMemoryEventStore struct {
	mu sync.RWMutex

	byWorld map[string][]spec.Event
	seenID  map[string]struct{}
	byID    map[string]map[string]spec.Event // world_id -> event_id -> event
}

func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		byWorld: map[string][]spec.Event{},
		seenID:  map[string]struct{}{},
		byID:    map[string]map[string]spec.Event{},
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
	if _, ok := s.byID[e.WorldID]; !ok {
		s.byID[e.WorldID] = map[string]spec.Event{}
	}
	s.byID[e.WorldID][e.EventID] = e

	// Keep per-world slice sorted by (ts asc, event_id asc) so queries are stable.
	//
	// Optimization: events are usually produced in chronological order, so we can skip
	// the O(n log n) full sort for the common append-at-end case. We only sort if the
	// last append violated ordering (ts/event_id regression).
	list := s.byWorld[e.WorldID]
	n := len(list)
	if n >= 2 {
		prev := list[n-2]
		cur := list[n-1]
		inOrder := prev.Ts < cur.Ts || (prev.Ts == cur.Ts && prev.EventID <= cur.EventID)
		if !inOrder {
			sort.Slice(list, func(i, j int) bool {
				a := list[i]
				b := list[j]
				if a.Ts != b.Ts {
					return a.Ts < b.Ts
				}
				return a.EventID < b.EventID
			})
			s.byWorld[e.WorldID] = list
		}
	}

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

func (s *InMemoryEventStore) GetByID(worldID, eventID string) (spec.Event, bool, error) {
	if worldID == "" || eventID == "" {
		return spec.Event{}, false, fmt.Errorf("world_id and event_id are required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.byID[worldID]
	if m == nil {
		return spec.Event{}, false, nil
	}
	e, ok := m[eventID]
	return e, ok, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
