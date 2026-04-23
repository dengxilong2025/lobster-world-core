package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

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

