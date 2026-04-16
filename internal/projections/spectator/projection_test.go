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

