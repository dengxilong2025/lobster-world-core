package sim

import (
	"testing"

	"lobster-world-core/internal/events/store"
)

func TestWorldStep_AppliesNaturalEvolutionWhenIdle(t *testing.T) {
	t.Parallel()

	es := store.NewInMemoryEventStore()
	w := newWorld("w_evo", 0, es, nil, 10)

	// Force an "idle but unstable" world.
	w.mu.Lock()
	w.state.Food = 0
	w.state.Population = 10
	w.state.Trust = 10
	w.state.Order = 10
	w.state.Conflict = 80
	w.mu.Unlock()

	// Evolution is emitted only after several consecutive idle ticks.
	for i := 0; i < 5; i++ {
		w.step()
	}

	// Should emit an evolution narrative event even when no intents.
	evs, err := es.Query(store.Query{WorldID: "w_evo", SinceTs: 0, Limit: 50})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	found := false
	for _, e := range evs {
		if e.Type == "world_evolved" {
			found = true
			if e.Delta == nil {
				t.Fatalf("expected delta in world_evolved")
			}
			if len(e.Trace) == 0 {
				t.Fatalf("expected trace in world_evolved")
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected world_evolved event")
	}

	st := w.status()
	if st.State.Population >= 10 {
		t.Fatalf("expected population to decrease from evolution, got %d", st.State.Population)
	}
}
