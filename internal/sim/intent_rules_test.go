package sim

import (
	"testing"

	"lobster-world-core/internal/events/store"
)

func TestIntentRules_MapsGoalsToDeltas(t *testing.T) {
	t.Parallel()

	es := store.NewInMemoryEventStore()
	w := newWorld("w1", 0, es, nil, 10, 256)

	// Submit an intent that should affect food.
	qi := queuedIntent{
		ID:     "int_1",
		Intent: Intent{Goal: "去狩猎获取食物"},
		Ack:    make(chan error, 1),
	}
	w.handleIntent(qi)
	w.step()

	evs, err := es.Query(store.Query{WorldID: "w1", SinceTs: 0, Limit: 50})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	var doneFound bool
	for _, e := range evs {
		if e.Type == "action_completed" {
			doneFound = true
			if e.Delta == nil {
				t.Fatalf("expected delta")
			}
			if v, ok := e.Delta["food"]; !ok || v.(int64) <= 0 {
				t.Fatalf("expected food delta >0, got %#v", e.Delta)
			}
			if len(e.Trace) == 0 {
				t.Fatalf("expected trace for action_completed")
			}
		}
	}
	if !doneFound {
		t.Fatalf("expected action_completed event")
	}
}
