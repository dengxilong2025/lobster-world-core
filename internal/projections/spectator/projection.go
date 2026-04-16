package spectator

import (
	"sort"
	"sync"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
)

// Projection maintains a minimal spectator read model derived from the event log.
//
// v0 goal: provide "headline + hot_events" fast and deterministically.
// It is designed to be rebuilt from the EventStore (read models are disposable).
type Projection struct {
	mu sync.RWMutex

	es store.EventStore

	// per world cache of recent events, sorted by (ts desc, event_id desc)
	recent map[string][]spec.Event
	limit  int
}

func (p *Projection) Entity(worldID, entityID string, eventLimit int) (EntityPage, error) {
	if eventLimit <= 0 {
		eventLimit = 20
	}
	if err := p.EnsureLoaded(worldID); err != nil {
		return EntityPage{}, err
	}

	p.mu.RLock()
	list := append([]spec.Event{}, p.recent[worldID]...)
	p.mu.RUnlock()

	// Recent events affecting the entity:
	recent := make([]spec.Event, 0, eventLimit)
	for _, e := range list {
		if e.EntityID == entityID {
			recent = append(recent, e)
			if len(recent) >= eventLimit {
				break
			}
		}
	}

	// Relations derived from world-level events (v0 heuristic):
	// - alliance_formed(actor0, actor1) => ally between each
	// - betrayal(actor0, actor1) => enemy between each
	relMap := map[string]Relation{}
	for _, e := range list {
		if e.Scope != "world" {
			continue
		}
		if len(e.Actors) < 2 {
			continue
		}
		a := e.Actors[0]
		b := e.Actors[1]
		if a != entityID && b != entityID {
			continue
		}
		other := a
		if a == entityID {
			other = b
		}
		switch e.Type {
		case "alliance_formed":
			relMap[other] = Relation{To: other, Type: "ally", Strength: 1}
		case "betrayal", "war_started":
			relMap[other] = Relation{To: other, Type: "enemy", Strength: 1}
		}
	}
	relations := make([]Relation, 0, len(relMap))
	for _, r := range relMap {
		relations = append(relations, r)
	}
	sort.Slice(relations, func(i, j int) bool { return relations[i].To < relations[j].To })

	return EntityPage{Relations: relations, RecentEvents: recent}, nil
}

type Options struct {
	EventStore store.EventStore
	Limit     int
}

func New(opts Options) *Projection {
	limit := opts.Limit
	if limit <= 0 {
		limit = 200
	}
	return &Projection{
		es:     opts.EventStore,
		recent: map[string][]spec.Event{},
		limit:  limit,
	}
}

// Apply ingests a new event into the read model (best-effort).
func (p *Projection) Apply(e spec.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	list := append(p.recent[e.WorldID], e)
	sort.Slice(list, func(i, j int) bool {
		if list[i].Ts != list[j].Ts {
			return list[i].Ts > list[j].Ts
		}
		return list[i].EventID > list[j].EventID
	})
	if len(list) > p.limit {
		list = list[:p.limit]
	}
	p.recent[e.WorldID] = list
}

// EnsureLoaded lazily loads recent events from the event store if the cache is empty.
// This supports process restarts and avoids relying solely on live event delivery.
func (p *Projection) EnsureLoaded(worldID string) error {
	p.mu.RLock()
	_, ok := p.recent[worldID]
	p.mu.RUnlock()
	if ok {
		return nil
	}
	if p.es == nil {
		return nil
	}

	events, err := p.es.Query(store.Query{WorldID: worldID, SinceTs: 0, Limit: p.limit})
	if err != nil {
		return err
	}

	// Query returns ts asc; store in ts desc.
	sort.Slice(events, func(i, j int) bool {
		if events[i].Ts != events[j].Ts {
			return events[i].Ts > events[j].Ts
		}
		return events[i].EventID > events[j].EventID
	})

	p.mu.Lock()
	p.recent[worldID] = events
	p.mu.Unlock()
	return nil
}

type Home struct {
	Headline  *spec.Event
	HotEvents []spec.Event
}

type EntityPage struct {
	Relations   []Relation
	RecentEvents []spec.Event
}

type Relation struct {
	To       string
	Type     string
	Strength int // v0: fixed 1
}

// Home returns the spectator home model derived from recent events.
func (p *Projection) Home(worldID string, hotLimit int) (Home, error) {
	if hotLimit <= 0 {
		hotLimit = 10
	}
	if err := p.EnsureLoaded(worldID); err != nil {
		return Home{}, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	list := p.recent[worldID]
	if len(list) == 0 {
		return Home{Headline: nil, HotEvents: []spec.Event{}}, nil
	}
	hl := &list[0]

	// Compute hotness based on type weight + recency decay.
	now := hl.Ts
	type scored struct {
		e     spec.Event
		score int64
	}
	scoredList := make([]scored, 0, len(list))
	for _, e := range list {
		age := now - e.Ts
		scoredList = append(scoredList, scored{e: e, score: scoreEvent(e.Type, age)})
	}
	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].score != scoredList[j].score {
			return scoredList[i].score > scoredList[j].score
		}
		if scoredList[i].e.Ts != scoredList[j].e.Ts {
			return scoredList[i].e.Ts > scoredList[j].e.Ts
		}
		return scoredList[i].e.EventID > scoredList[j].e.EventID
	})

	n := hotLimit
	if len(scoredList) < n {
		n = len(scoredList)
	}
	out := make([]spec.Event, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, scoredList[i].e)
	}
	return Home{Headline: hl, HotEvents: out}, nil
}
