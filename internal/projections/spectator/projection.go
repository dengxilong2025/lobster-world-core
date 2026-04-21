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

	// EnsureLoaded singleflight: avoid redundant rebuilds per world_id under concurrency.
	loading map[string]*loadState

	// bootstrapped marks worlds that have been rebuilt from EventStore at least once.
	// This is distinct from "recent has key": live Apply() events can arrive before any
	// initial load, and we still want to backfill history from the store.
	bootstrapped map[string]bool

	// per world cache of recent events, sorted by (ts desc, event_id desc)
	recent map[string][]spec.Event
	// per world relations cache: world -> entity -> other -> relation
	relations map[string]map[string]map[string]Relation
	// per world relation reasons cache: world -> entity -> other -> reason
	relationReasons map[string]map[string]map[string]RelationReason
	limit  int
	hotHalfLifeTicks int64
}

type loadState struct {
	done chan struct{}
	err  error
}

type Options struct {
	EventStore store.EventStore
	Limit     int
	// HotHalfLifeTicks controls recency decay in "hot events" ranking for sim-generated events.
	// Default 360 ticks (≈30 minutes if tickInterval is 5s).
	HotHalfLifeTicks int64
}

func New(opts Options) *Projection {
	limit := opts.Limit
	if limit <= 0 {
		limit = 200
	}
	hl := opts.HotHalfLifeTicks
	if hl <= 0 {
		hl = 360
	}
	return &Projection{
		es:     opts.EventStore,
		loading: map[string]*loadState{},
		bootstrapped: map[string]bool{},
		recent: map[string][]spec.Event{},
		relations: map[string]map[string]map[string]Relation{},
		relationReasons: map[string]map[string]map[string]RelationReason{},
		limit:  limit,
		hotHalfLifeTicks: hl,
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

	// Maintain relations incrementally (high cohesion: relation logic stays inside projection).
	p.applyRelationLocked(e)
}

// EnsureLoaded lazily loads recent events from the event store if the cache is empty.
// This supports process restarts and avoids relying solely on live event delivery.
func (p *Projection) EnsureLoaded(worldID string) error {
	if p.es == nil {
		return nil
	}

	// Fast path: already loaded.
	p.mu.RLock()
	if p.bootstrapped[worldID] {
		p.mu.RUnlock()
		return nil
	}
	// If another goroutine is loading this world, wait.
	if st := p.loading[worldID]; st != nil {
		p.mu.RUnlock()
		<-st.done
		return st.err
	}
	p.mu.RUnlock()

	// Slow path: acquire singleflight token (double-check under lock).
	p.mu.Lock()
	if p.bootstrapped[worldID] {
		p.mu.Unlock()
		return nil
	}
	if st := p.loading[worldID]; st != nil {
		p.mu.Unlock()
		<-st.done
		return st.err
	}
	st := &loadState{done: make(chan struct{})}
	p.loading[worldID] = st
	p.mu.Unlock()

	// Perform load outside lock.
	events, err := p.es.Query(store.Query{WorldID: worldID, SinceTs: 0, Limit: p.limit})
	var raw []spec.Event
	if err == nil {
		// Build relation cache deterministically from event log (ts asc from store.Query).
		// Keep a ts-asc copy for relation rebuild.
		raw = append([]spec.Event{}, events...)
	}

	// Publish result, unblock waiters.
	p.mu.Lock()
	delete(p.loading, worldID)
	st.err = err
	close(st.done)
	if err != nil {
		p.mu.Unlock()
		return err
	}

	// Merge with any live events already applied before the initial load (important when
	// EventStore persists across process restarts).
	merged := make(map[string]spec.Event, len(events)+len(p.recent[worldID]))
	for _, e := range events {
		if e.EventID == "" {
			continue
		}
		merged[e.EventID] = e
	}
	for _, e := range p.recent[worldID] {
		if e.EventID == "" {
			continue
		}
		merged[e.EventID] = e
	}
	combined := make([]spec.Event, 0, len(merged))
	for _, e := range merged {
		combined = append(combined, e)
	}

	// Store in ts desc.
	sort.Slice(combined, func(i, j int) bool {
		if combined[i].Ts != combined[j].Ts {
			return combined[i].Ts > combined[j].Ts
		}
		return combined[i].EventID > combined[j].EventID
	})
	if len(combined) > p.limit {
		combined = combined[:p.limit]
	}
	p.recent[worldID] = combined

	p.relations[worldID] = map[string]map[string]Relation{}
	p.relationReasons[worldID] = map[string]map[string]RelationReason{}
	// Rebuild relations deterministically from the merged view.
	// Apply in ts asc order.
	sort.Slice(raw, func(i, j int) bool {
		if raw[i].Ts != raw[j].Ts {
			return raw[i].Ts < raw[j].Ts
		}
		return raw[i].EventID < raw[j].EventID
	})
	for _, e := range raw {
		p.applyRelationLocked(e)
	}
	// Also apply relation updates from any live events that were not in raw.
	// (raw comes from store.Query; combined may contain additional events from Apply().)
	if len(combined) > 0 {
		// combined is desc; walk reverse for asc.
		for i := len(combined) - 1; i >= 0; i-- {
			e := combined[i]
			// Skip if raw already contained the event.
			// (OK to be O(n^2) since limit is small; but keep it deterministic.)
			found := false
			for _, re := range raw {
				if re.EventID == e.EventID {
					found = true
					break
				}
			}
			if !found {
				p.applyRelationLocked(e)
			}
		}
	}

	p.bootstrapped[worldID] = true
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
	WhyStrong   []string
	NextRisk    []string
	RelationReasons []RelationReason
}

type Relation struct {
	To       string
	Type     string
	Strength int // v0: fixed 1
}

type RelationReason struct {
	To      string `json:"to"`
	Type    string `json:"type"`
	EventID string `json:"event_id"`
	Note    string `json:"note"`
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
	relWorld := p.relations[worldID]
	reasonWorld := p.relationReasons[worldID]
	p.mu.RUnlock()

	// Recent events affecting the entity (entity-scoped events).
	recent := make([]spec.Event, 0, eventLimit)
	for _, e := range list {
		if e.EntityID == entityID {
			recent = append(recent, e)
			if len(recent) >= eventLimit {
				break
			}
		}
	}

	// Why strong: derive short explanations from recent entity events + allies.
	whyStrong := make([]string, 0, 3)
	for _, e := range recent {
		switch e.Type {
		case "skill_gained":
			whyStrong = append(whyStrong, "关键技能："+e.Narrative)
		case "milestone_reached":
			whyStrong = append(whyStrong, "里程碑："+e.Narrative)
		}
		if len(whyStrong) >= 3 {
			break
		}
	}

	relMap := map[string]Relation{}
	if relWorld != nil && relWorld[entityID] != nil {
		for other, r := range relWorld[entityID] {
			relMap[other] = r
		}
	}
	relations := make([]Relation, 0, len(relMap))
	for _, r := range relMap {
		relations = append(relations, r)
	}
	sort.Slice(relations, func(i, j int) bool { return relations[i].To < relations[j].To })

	reasonMap := map[string]RelationReason{}
	if reasonWorld != nil && reasonWorld[entityID] != nil {
		for other, rr := range reasonWorld[entityID] {
			reasonMap[other] = rr
		}
	}
	reasons := make([]RelationReason, 0, len(reasonMap))
	for _, rr := range reasonMap {
		reasons = append(reasons, rr)
	}
	sort.Slice(reasons, func(i, j int) bool { return reasons[i].To < reasons[j].To })

	// Add betrayal-based explanations (MVP "解说" for why_strong).
	if len(whyStrong) < 3 {
		for _, e := range list {
			if e.Scope != "world" || e.Type != "betrayal" || len(e.Actors) < 2 {
				continue
			}
			a := e.Actors[0]
			b := e.Actors[1]
			if a == entityID {
				whyStrong = append(whyStrong, "信誉受损："+e.Narrative)
				break
			}
			if b == entityID {
				whyStrong = append(whyStrong, "遭遇背叛："+e.Narrative)
				break
			}
		}
	}

	// Add shock-period explanation (MVP).
	if len(whyStrong) < 3 {
		for _, e := range list {
			if e.Scope == "world" && e.Type == "shock_started" {
				whyStrong = append(whyStrong, "冲击期："+e.Narrative)
				break
			}
		}
	}

	// Add shock-warning explanation (MVP).
	if len(whyStrong) < 3 {
		for _, e := range list {
			if e.Scope == "world" && e.Type == "shock_warning" {
				whyStrong = append(whyStrong, "冲击预兆："+e.Narrative)
				break
			}
		}
	}

	// If no "why strong" from events, fall back to ally relation as explanation.
	if len(whyStrong) == 0 {
		for _, r := range relations {
			if r.Type == "ally" {
				whyStrong = append(whyStrong, "盟友支持："+r.To)
				break
			}
		}
	}

	// Next risks: derived from world shock + nearby conflict with this entity.
	nextRisk := make([]string, 0, 3)
	for _, e := range list {
		// world-level shocks affect everyone
		if e.Scope == "world" && e.Type == "shock_started" {
			nextRisk = append(nextRisk, "冲击期："+e.Narrative)
			break
		}
		if e.Scope == "world" && e.Type == "shock_warning" {
			nextRisk = append(nextRisk, "冲击预兆："+e.Narrative)
			break
		}
	}
	for _, e := range list {
		if e.Scope != "world" || len(e.Actors) < 2 {
			continue
		}
		a := e.Actors[0]
		b := e.Actors[1]
		if a != entityID && b != entityID {
			continue
		}
		if e.Type == "betrayal" {
			nextRisk = append(nextRisk, "背叛已发生："+e.Narrative)
			break
		}
		if e.Type == "war_started" || e.Type == "battle_resolved" {
			nextRisk = append(nextRisk, "冲突升级："+e.Narrative)
			break
		}
	}

	return EntityPage{Relations: relations, RelationReasons: reasons, RecentEvents: recent, WhyStrong: whyStrong, NextRisk: nextRisk}, nil
}

func (p *Projection) applyRelationLocked(e spec.Event) {
	if e.WorldID == "" {
		return
	}
	if e.Scope != "world" {
		return
	}
	if len(e.Actors) < 2 {
		return
	}
	a := e.Actors[0]
	b := e.Actors[1]

	relType := ""
	switch e.Type {
	case "alliance_formed", "trade_agreement", "treaty_signed":
		relType = "ally"
	case "betrayal", "war_started", "battle_resolved":
		relType = "enemy"
	default:
		return
	}

	if p.relations[e.WorldID] == nil {
		p.relations[e.WorldID] = map[string]map[string]Relation{}
	}
	if p.relationReasons[e.WorldID] == nil {
		p.relationReasons[e.WorldID] = map[string]map[string]RelationReason{}
	}
	if p.relations[e.WorldID][a] == nil {
		p.relations[e.WorldID][a] = map[string]Relation{}
	}
	if p.relations[e.WorldID][b] == nil {
		p.relations[e.WorldID][b] = map[string]Relation{}
	}
	if p.relationReasons[e.WorldID][a] == nil {
		p.relationReasons[e.WorldID][a] = map[string]RelationReason{}
	}
	if p.relationReasons[e.WorldID][b] == nil {
		p.relationReasons[e.WorldID][b] = map[string]RelationReason{}
	}
	p.relations[e.WorldID][a][b] = Relation{To: b, Type: relType, Strength: 1}
	p.relations[e.WorldID][b][a] = Relation{To: a, Type: relType, Strength: 1}

	// Keep the last reason deterministic from the event log (later event overrides earlier ones).
	p.relationReasons[e.WorldID][a][b] = RelationReason{To: b, Type: relType, EventID: e.EventID, Note: e.Narrative}
	p.relationReasons[e.WorldID][b][a] = RelationReason{To: a, Type: relType, EventID: e.EventID, Note: e.Narrative}
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
	nowTick := hl.Tick
	type scored struct {
		e     spec.Event
		score int64
	}
	scoredList := make([]scored, 0, len(list))
	for _, e := range list {
		// Prefer tick-based decay for deterministic sim events.
		if nowTick > 0 && e.Tick > 0 {
			ageTicks := nowTick - e.Tick
			scoredList = append(scoredList, scored{e: e, score: scoreEventTicks(e.Type, ageTicks, p.hotHalfLifeTicks)})
			continue
		}
		// Fallback to ts-based decay for events without tick info.
		age := now - e.Ts
		scoredList = append(scoredList, scored{e: e, score: scoreEventSec(e.Type, age, 1800)})
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
