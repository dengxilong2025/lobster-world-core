package integration

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func getMetricsMap(t *testing.T, baseURL string) map[string]any {
	t.Helper()
	resp, err := http.Get(baseURL + "/api/v0/debug/metrics")
	if err != nil {
		t.Fatalf("GET debug/metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	mm, _ := out["metrics"].(map[string]any)
	if mm == nil {
		t.Fatalf("expected metrics object, got %#v", out["metrics"])
	}
	return mm
}

func metricInt64(t *testing.T, mm map[string]any, key string) int64 {
	t.Helper()
	v, ok := mm[key]
	if !ok {
		t.Fatalf("missing metric key %q in %#v", key, mm)
	}
	// encoding/json decodes numbers to float64 in map[string]any
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("metric %q expected number, got %T=%#v", key, v, v)
	}
	return int64(f)
}

func TestDebugMetricsV1_ContainsExpectedFields(t *testing.T) {
	t.Parallel()

	app := gateway.NewApp()
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	mm := getMetricsMap(t, s.URL)

	// EventStore
	_ = metricInt64(t, mm, "eventstore_append_total")
	_ = metricInt64(t, mm, "eventstore_append_errors_total")

	// Replay export/highlight
	_ = metricInt64(t, mm, "replay_export_total")
	_ = metricInt64(t, mm, "replay_export_errors_total")
	_ = metricInt64(t, mm, "replay_export_time_ms_total")
	_ = metricInt64(t, mm, "replay_export_bytes_total")
	_ = metricInt64(t, mm, "replay_highlight_total")
	_ = metricInt64(t, mm, "replay_highlight_errors_total")
	_ = metricInt64(t, mm, "replay_highlight_time_ms_total")

	// SSE
	_ = metricInt64(t, mm, "sse_connections_current")
	_ = metricInt64(t, mm, "sse_connections_total")
	_ = metricInt64(t, mm, "sse_disconnects_total")
	_ = metricInt64(t, mm, "sse_data_messages_total")
}

func TestDebugMetricsV1_SSEConnectionsGaugeChanges(t *testing.T) {
	t.Parallel()

	app := gateway.NewApp()
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/v0/events/stream?world_id=w1", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("connect stream: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Read the first line so we know the stream is actually established.
	br := bufio.NewReader(resp.Body)
	_, _ = br.ReadString('\n')

	m1 := getMetricsMap(t, s.URL)
	if cur := metricInt64(t, m1, "sse_connections_current"); cur < 1 {
		t.Fatalf("expected sse_connections_current>=1, got %d", cur)
	}

	_ = resp.Body.Close()
	time.Sleep(50 * time.Millisecond)

	m2 := getMetricsMap(t, s.URL)
	if cur := metricInt64(t, m2, "sse_connections_current"); cur != 0 {
		t.Fatalf("expected sse_connections_current==0 after close, got %d", cur)
	}
}

func TestDebugMetricsV1_ReplayExportCountersIncrease(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		Seed:         123,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_metrics_export"

	// Create a world and some events.
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "贸易：开通商路"})
	r, err := http.Post(s.URL+"/api/v0/intents", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	_ = r.Body.Close()
	time.Sleep(200 * time.Millisecond)

	before := getMetricsMap(t, s.URL)
	bExport := metricInt64(t, before, "replay_export_total")

	er, err := http.Get(s.URL + "/api/v0/replay/export?world_id=" + worldID + "&limit=5000")
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	_ = er.Body.Close()
	if er.StatusCode != http.StatusOK {
		t.Fatalf("export status=%d", er.StatusCode)
	}

	after := getMetricsMap(t, s.URL)
	aExport := metricInt64(t, after, "replay_export_total")
	if aExport <= bExport {
		t.Fatalf("expected replay_export_total to increase, before=%d after=%d", bExport, aExport)
	}
	if metricInt64(t, after, "replay_export_bytes_total") <= 0 {
		t.Fatalf("expected replay_export_bytes_total > 0")
	}
	if metricInt64(t, after, "replay_export_time_ms_total") <= 0 {
		t.Fatalf("expected replay_export_time_ms_total > 0")
	}
}

func TestDebugMetricsV1_EventStoreAppendCountersIncrease(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		Seed:         123,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_metrics_append"
	before := getMetricsMap(t, s.URL)
	b := metricInt64(t, before, "eventstore_append_total")

	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
	r, err := http.Post(s.URL+"/api/v0/intents", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	_ = r.Body.Close()
	time.Sleep(200 * time.Millisecond)

	after := getMetricsMap(t, s.URL)
	a := metricInt64(t, after, "eventstore_append_total")
	if a <= b {
		t.Fatalf("expected eventstore_append_total to increase, before=%d after=%d", b, a)
	}
	// ensure append errors exists (not necessarily >0).
	_ = metricInt64(t, after, "eventstore_append_errors_total")
}

func TestDebugMetricsV1_BusyTotalStillTracked(t *testing.T) {
	t.Parallel()

	// Use a low max queue to force 503 BUSY quickly.
	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		MaxIntentQueue: 1,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_busy_metrics"
	// Flood enough requests to potentially trigger backpressure.
	for i := 0; i < 50; i++ {
		body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
		resp, _ := http.Post(s.URL+"/api/v0/intents", "application/json", strings.NewReader(string(body)))
		if resp != nil {
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
		}
	}

	m := getMetricsMap(t, s.URL)
	_ = metricInt64(t, m, "busy_total")
}

