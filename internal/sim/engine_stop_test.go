package sim

import (
	"testing"
	"time"

	"lobster-world-core/internal/events/store"
)

func TestEngineStop_StopsWorldTicksAndPreventsNewIntents(t *testing.T) {
	t.Parallel()

	es := store.NewInMemoryEventStore()
	e := New(Options{EventStore: es, TickInterval: 5 * time.Millisecond})

	worldID := "w_stop"
	e.EnsureWorld(worldID)

	// Let it tick a bit.
	time.Sleep(30 * time.Millisecond)
	st1, ok := e.GetStatus(worldID)
	if !ok {
		t.Fatalf("expected status ok")
	}
	if st1.Tick <= 0 {
		t.Fatalf("expected tick > 0, got %d", st1.Tick)
	}

	e.Stop()
	// Stop should be idempotent.
	e.Stop()

	// Tick should stop advancing (allow one in-flight tick before stop takes effect).
	time.Sleep(10 * time.Millisecond)
	stStop, ok := e.GetStatus(worldID)
	if !ok {
		t.Fatalf("expected status ok after stop")
	}
	frozen := stStop.Tick
	time.Sleep(30 * time.Millisecond)
	st2, ok := e.GetStatus(worldID)
	if !ok {
		t.Fatalf("expected status ok after stop")
	}
	if st2.Tick != frozen {
		t.Fatalf("expected tick frozen at %d, got %d", frozen, st2.Tick)
	}

	// After Stop, new intents should not be accepted.
	id, err := e.SubmitIntent(worldID, Intent{Goal: "不会被执行"})
	if err == nil || id != "" {
		t.Fatalf("expected error and empty intentID after stop, got id=%q err=%v", id, err)
	}
}
