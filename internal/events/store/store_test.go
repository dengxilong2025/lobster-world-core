package store

import (
	"testing"

	"lobster-world-core/internal/events/spec"
)

func TestInMemoryEventStore_AppendAndQueryByWorldID(t *testing.T) {
	t.Parallel()

	s := NewInMemoryEventStore()

	e1 := spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_1",
		Ts:            1710000001,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "shock_warning",
		Actors:        []string{"lobster_1"},
		Narrative:     "n1",
	}
	e2 := e1
	e2.EventID = "evt_2"
	e2.Ts = 1710000002
	e2.Narrative = "n2"

	if err := s.Append(e1); err != nil {
		t.Fatalf("append e1: %v", err)
	}
	if err := s.Append(e2); err != nil {
		t.Fatalf("append e2: %v", err)
	}

	got, err := s.Query(Query{
		WorldID:  "w1",
		SinceTs:  0,
		Limit:    10,
		EntityID: "",
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].EventID != "evt_1" || got[1].EventID != "evt_2" {
		t.Fatalf("unexpected order: %v, %v", got[0].EventID, got[1].EventID)
	}
}

func TestInMemoryEventStore_QueryFiltersBySinceTs(t *testing.T) {
	t.Parallel()

	s := NewInMemoryEventStore()

	base := spec.Event{
		SchemaVersion: 1,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "x",
		Actors:        []string{"a"},
		Narrative:     "n",
	}

	e1 := base
	e1.EventID = "evt_1"
	e1.Ts = 10
	e2 := base
	e2.EventID = "evt_2"
	e2.Ts = 20

	_ = s.Append(e1)
	_ = s.Append(e2)

	got, err := s.Query(Query{WorldID: "w1", SinceTs: 15, Limit: 10})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 || got[0].EventID != "evt_2" {
		t.Fatalf("expected only evt_2, got %#v", got)
	}
}

func TestInMemoryEventStore_RejectsDuplicateEventID(t *testing.T) {
	t.Parallel()

	s := NewInMemoryEventStore()
	e := spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_dup",
		Ts:            1,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "x",
		Actors:        []string{"a"},
		Narrative:     "n",
	}

	if err := s.Append(e); err != nil {
		t.Fatalf("append first: %v", err)
	}
	if err := s.Append(e); err == nil {
		t.Fatalf("expected duplicate error, got nil")
	}
}

func TestInMemoryEventStore_GetByID(t *testing.T) {
	t.Parallel()

	s := NewInMemoryEventStore()
	e := spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_get",
		Ts:            1,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "x",
		Actors:        []string{"a"},
		Narrative:     "n",
	}
	if err := s.Append(e); err != nil {
		t.Fatalf("append: %v", err)
	}

	got, ok, err := s.GetByID("w1", "evt_get")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got.EventID != "evt_get" || got.WorldID != "w1" {
		t.Fatalf("unexpected event: %#v", got)
	}

	_, ok, err = s.GetByID("w1", "missing")
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing")
	}
}
