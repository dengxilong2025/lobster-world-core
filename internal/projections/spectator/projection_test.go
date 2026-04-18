package spectator

import (
	"testing"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
)

func TestProjectionHome_UsesNewestAsHeadline(t *testing.T) {
	t.Parallel()

	es := store.NewInMemoryEventStore()
	_ = es.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_1",
		Ts:            10,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "a",
		Actors:        []string{"x"},
		Narrative:     "n1",
	})
	_ = es.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_2",
		Ts:            20,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "b",
		Actors:        []string{"x"},
		Narrative:     "n2",
	})

	p := New(Options{EventStore: es, Limit: 50})
	home, err := p.Home("w1", 10)
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	if home.Headline == nil || home.Headline.EventID != "evt_2" {
		t.Fatalf("expected headline evt_2, got %#v", home.Headline)
	}
	if len(home.HotEvents) != 2 {
		t.Fatalf("expected 2 hot events, got %d", len(home.HotEvents))
	}
}

func TestProjectionHome_RanksByHotnessWeightThenRecency(t *testing.T) {
	t.Parallel()

	es := store.NewInMemoryEventStore()

	// Newer but low weight.
	_ = es.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_low",
		Ts:            100,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "intent_accepted",
		Actors:        []string{"x"},
		Narrative:     "low",
	})
	// Slightly older but high weight.
	_ = es.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_high",
		Ts:            90,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "betrayal",
		Actors:        []string{"x"},
		Narrative:     "high",
	})

	p := New(Options{EventStore: es, Limit: 50})
	home, err := p.Home("w1", 2)
	if err != nil {
		t.Fatalf("home: %v", err)
	}

	if home.Headline == nil || home.Headline.EventID != "evt_low" {
		t.Fatalf("expected headline newest evt_low, got %#v", home.Headline)
	}
	if len(home.HotEvents) != 2 {
		t.Fatalf("expected 2 hot events, got %d", len(home.HotEvents))
	}
	if home.HotEvents[0].EventID != "evt_high" {
		t.Fatalf("expected hot[0]=evt_high, got %s", home.HotEvents[0].EventID)
	}
}

func TestProjectionHome_UsesTickBasedHalfLifeForSimEvents(t *testing.T) {
	t.Parallel()

	es := store.NewInMemoryEventStore()

	// Headline is the newest by ts, and has tick information.
	_ = es.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_new",
		Ts:            1000,
		Tick:          1000,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "intent_accepted",
		Actors:        []string{"x"},
		Narrative:     "new",
	})
	// Very old betrayal with high weight, but should decay heavily when halfLifeTicks is small.
	_ = es.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_old_betrayal",
		Ts:            1,
		Tick:          1,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "betrayal",
		Actors:        []string{"x", "y"},
		Narrative:     "old",
	})

	p := New(Options{EventStore: es, Limit: 50, HotHalfLifeTicks: 10})
	home, err := p.Home("w1", 2)
	if err != nil {
		t.Fatalf("home: %v", err)
	}
	if len(home.HotEvents) != 2 {
		t.Fatalf("expected 2 hot events, got %d", len(home.HotEvents))
	}
	// With tiny half-life, the old betrayal should decay below the new event.
	if home.HotEvents[0].EventID != "evt_new" {
		t.Fatalf("expected hot[0]=evt_new with tick decay, got %s", home.HotEvents[0].EventID)
	}
}

func TestProjectionEntity_RelationsAreDeterministicFromEventLog(t *testing.T) {
	t.Parallel()

	es := store.NewInMemoryEventStore()
	_ = es.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_1",
		Ts:            10,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "alliance_formed",
		Actors:        []string{"nation_a", "nation_b"},
		Narrative:     "A与B结盟",
	})
	// Later betrayal should override relation to enemy.
	_ = es.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_2",
		Ts:            20,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "betrayal",
		Actors:        []string{"nation_a", "nation_b"},
		Narrative:     "A背叛B",
	})

	p := New(Options{EventStore: es, Limit: 50})
	page, err := p.Entity("w1", "nation_a", 10)
	if err != nil {
		t.Fatalf("entity: %v", err)
	}
	if len(page.Relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(page.Relations))
	}
	if page.Relations[0].To != "nation_b" || page.Relations[0].Type != "enemy" {
		t.Fatalf("expected enemy relation to nation_b, got %#v", page.Relations[0])
	}
}
