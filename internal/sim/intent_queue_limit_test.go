package sim

import (
	"errors"
	"testing"
	"time"

	"lobster-world-core/internal/events/store"
)

func TestEngine_SubmitIntentReturnsBusyWhenQueueFull(t *testing.T) {
	t.Parallel()

	es := store.NewInMemoryEventStore()
	e := New(Options{
		EventStore:     es,
		TickInterval:   5 * time.Second, // keep queue from draining during test
		MaxIntentQueue: 1,
	})

	worldID := "w_q"
	_, err := e.SubmitIntent(worldID, Intent{Goal: "a"})
	if err != nil {
		t.Fatalf("first intent should succeed, got %v", err)
	}
	_, err = e.SubmitIntent(worldID, Intent{Goal: "b"})
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", err)
	}
}

