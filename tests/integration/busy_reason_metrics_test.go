package integration

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"net/http"
	"net/http/httptest"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
	"lobster-world-core/internal/gateway"
	"lobster-world-core/internal/sim"
)

type blockingEventStore struct {
	startedOnce sync.Once
	started     chan struct{}
	unblock     chan struct{}
}

func newBlockingEventStore() *blockingEventStore {
	return &blockingEventStore{
		started: make(chan struct{}),
		unblock: make(chan struct{}),
	}
}

func (b *blockingEventStore) Append(e spec.Event) error {
	b.startedOnce.Do(func() { close(b.started) })
	<-b.unblock
	return nil
}
func (b *blockingEventStore) Query(q store.Query) ([]spec.Event, error) { return []spec.Event{}, nil }
func (b *blockingEventStore) GetByID(worldID, eventID string) (spec.Event, bool, error) {
	return spec.Event{}, false, nil
}

func TestBusyReasonMetrics_PendingQueueFull(t *testing.T) {
	t.Parallel()

	// Large tick interval so the queue doesn't drain.
	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval:        5 * time.Second,
		MaxIntentQueue:      1,
		IntentAcceptTimeout: 200 * time.Millisecond,
		Seed:               123,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_busy_reason_q"

	postIntent := func() *http.Response {
		body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
		r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST intents: %v", err)
		}
		return r
	}

	// First should be accepted.
	r1 := postIntent()
	_ = r1.Body.Close()
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("expected first 200, got %d", r1.StatusCode)
	}

	// Second should be rejected as BUSY due to pending queue limit.
	r2 := postIntent()
	_ = r2.Body.Close()
	if r2.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected second 503, got %d", r2.StatusCode)
	}

	mm := getMetricsMap(t, s.URL)
	by, ok := mm["busy_by_reason"].(map[string]any)
	if !ok || by == nil {
		t.Fatalf("expected busy_by_reason object, got %#v", mm["busy_by_reason"])
	}
	v, ok := by["pending_queue_full"].(float64)
	if !ok {
		t.Fatalf("expected pending_queue_full number, got %#v", by["pending_queue_full"])
	}
	if int64(v) < 1 {
		t.Fatalf("expected pending_queue_full>=1, got %d", int64(v))
	}
}

func TestBusyReasonMetrics_AcceptTimeout(t *testing.T) {
	// No t.Parallel(): uses timing and goroutines.
	bs := newBlockingEventStore()
	hub := stream.NewHub()

	// Unblock after a bit so background goroutine can finish.
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(bs.unblock)
	}()

	intentAcceptTimeout := 10 * time.Millisecond
	eng := sim.New(sim.Options{
		EventStore:          bs,
		Hub:                hub,
		TickInterval:        5 * time.Second,
		IntentAcceptTimeout: intentAcceptTimeout,
		MaxIntentQueue:      1024,
		// IntentChannelCap uses default (256) here.
	})
	t.Cleanup(func() { eng.Stop() })

	h := gateway.NewHandler(gateway.Options{
		EventStore: bs,
		Hub:        hub,
		Sim:        eng,
		Metrics:    gateway.NewMetrics(),
	})
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)

	body, _ := json.Marshal(map[string]any{"world_id": "w_busy_accept_timeout", "goal": "启动世界"})
	resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST intents: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}

	mm := getMetricsMap(t, s.URL)
	by, ok := mm["busy_by_reason"].(map[string]any)
	if !ok || by == nil {
		t.Fatalf("expected busy_by_reason object, got %#v", mm["busy_by_reason"])
	}
	v, ok := by["accept_timeout"].(float64)
	if !ok {
		t.Fatalf("expected accept_timeout number, got %#v", by["accept_timeout"])
	}
	if int64(v) < 1 {
		t.Fatalf("expected accept_timeout>=1, got %d", int64(v))
	}
}

func TestBusyReasonMetrics_IntentChFull(t *testing.T) {
	// No t.Parallel(): involves high concurrency; keep it stable.
	chCap := 0 // unbuffered; under burst concurrency, some sends will hit default -> intent_ch_full
	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval:     5 * time.Second,
		IntentChannelCap: &chCap,
		MaxIntentQueue:   1024,
		Seed:            123,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_busy_intent_ch"
	client := &http.Client{Timeout: 2 * time.Second}

	const n = 200
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
			resp, err := client.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
			if resp != nil {
				_ = resp.Body.Close()
			}
			_ = err
		}()
	}
	wg.Wait()

	mm := getMetricsMap(t, s.URL)
	by, ok := mm["busy_by_reason"].(map[string]any)
	if !ok || by == nil {
		t.Fatalf("expected busy_by_reason object, got %#v", mm["busy_by_reason"])
	}
	v, ok := by["intent_ch_full"].(float64)
	if !ok {
		t.Fatalf("expected intent_ch_full number, got %#v", by["intent_ch_full"])
	}
	if int64(v) < 1 {
		t.Fatalf("expected intent_ch_full>=1, got %d", int64(v))
	}
}
