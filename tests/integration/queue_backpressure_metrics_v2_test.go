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

func TestDebugMetricsV2Queue_AcceptWaitCountersIncrease(t *testing.T) {
	t.Parallel()
	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 5 * time.Millisecond, Seed: 123})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	before := getMetricsMap(t, s.URL)
	bCnt := metricInt64(t, before, "intent_accept_wait_count")

	worldID := "w_qv2_wait"
	for i := 0; i < 3; i++ {
		body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
		r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST intents: %v", err)
		}
		_ = r.Body.Close()
		if r.StatusCode != http.StatusOK {
			t.Fatalf("status=%d", r.StatusCode)
		}
	}
	time.Sleep(50 * time.Millisecond)

	after := getMetricsMap(t, s.URL)
	aCnt := metricInt64(t, after, "intent_accept_wait_count")
	if aCnt < bCnt+3 {
		t.Fatalf("expected accept_wait_count to increase by >=3, before=%d after=%d", bCnt, aCnt)
	}
	if metricInt64(t, after, "intent_accept_wait_ms_total") <= 0 {
		t.Fatalf("expected intent_accept_wait_ms_total > 0")
	}
}

func TestDebugMetricsV2Queue_WorldQueueStatsShowsPendingQueue(t *testing.T) {
	t.Parallel()

	// Large tick interval so the queue doesn't drain (world executes <=1 intent per tick).
	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 5 * time.Second, Seed: 123, MaxIntentQueue: 1024})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_qv2_depth"
	for i := 0; i < 10; i++ {
		body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
		r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST intents: %v", err)
		}
		_ = r.Body.Close()
		if r.StatusCode != http.StatusOK {
			t.Fatalf("status=%d", r.StatusCode)
		}
	}
	time.Sleep(50 * time.Millisecond)

	mm := getMetricsMap(t, s.URL)
	wqs, ok := mm["world_queue_stats"].(map[string]any)
	if !ok || wqs == nil {
		t.Fatalf("expected world_queue_stats object, got %#v", mm["world_queue_stats"])
	}
	ws, ok := wqs[worldID].(map[string]any)
	if !ok || ws == nil {
		t.Fatalf("expected world_id entry, got %#v", wqs[worldID])
	}
	pq, ok := ws["pending_queue_len"].(float64)
	if !ok {
		t.Fatalf("expected pending_queue_len number, got %#v", ws["pending_queue_len"])
	}
	if int64(pq) <= 0 {
		t.Fatalf("expected pending_queue_len>0, got %d (ws=%#v)", int64(pq), ws)
	}
}

