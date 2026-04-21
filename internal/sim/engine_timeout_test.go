package sim

import (
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
)

type blockingStore struct {
	unblock chan struct{}
}

func (b *blockingStore) Append(e spec.Event) error {
	<-b.unblock
	return nil
}

func (b *blockingStore) Query(q store.Query) ([]spec.Event, error) { return []spec.Event{}, nil }

func (b *blockingStore) GetByID(worldID, eventID string) (spec.Event, bool, error) {
	return spec.Event{}, false, nil
}

func TestEngine_SubmitIntent_UsesConfigurableAcceptTimeout(t *testing.T) {
	// No t.Parallel(): uses timing and goroutines.

	bs := &blockingStore{unblock: make(chan struct{})}
	hub := stream.NewHub()

	// Unblock after 50ms so the engine goroutine can finish even if SubmitIntent times out.
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(bs.unblock)
	}()

	e := New(Options{
		EventStore:          bs,
		Hub:                hub,
		TickInterval:        5 * time.Second,
		IntentAcceptTimeout: 10 * time.Millisecond,
	})
	t.Cleanup(func() { e.Stop() })

	start := time.Now()
	_, err := e.SubmitIntent("w1", Intent{Goal: "a"})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timeout waiting for intent acceptance") {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("expected timeout quickly, took %v", elapsed)
	}
}

