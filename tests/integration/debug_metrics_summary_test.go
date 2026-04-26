package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func TestDebugMetrics_IncludesSummary(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	resp, err := http.Get(s.URL + "/api/v0/debug/metrics")
	if err != nil {
		t.Fatalf("GET metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out struct {
		OK      bool                   `json:"ok"`
		Metrics map[string]interface{} `json:"metrics"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK {
		t.Fatalf("ok should be true")
	}

	sum, ok := out.Metrics["summary"].(map[string]interface{})
	if !ok || sum == nil {
		t.Fatalf("missing metrics.summary")
	}
	for _, k := range []string{"busy", "queue", "tick"} {
		if _, ok := sum[k]; !ok {
			t.Fatalf("missing summary.%s", k)
		}
	}

	// Regression: the original structured fields must remain present.
	if _, ok := out.Metrics["world_queue_stats"]; !ok {
		t.Fatalf("missing metrics.world_queue_stats")
	}
	if _, ok := out.Metrics["world_tick_stats"]; !ok {
		t.Fatalf("missing metrics.world_tick_stats")
	}
}

