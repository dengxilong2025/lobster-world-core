package integration

import (
	"net/http/httptest"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func TestDebugMetrics_TickStatsV1_TicksIncrease(t *testing.T) {
	t.Parallel()
	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		Seed:         123,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_tick_stats"
	app.Sim.EnsureWorld(worldID)
	time.Sleep(50 * time.Millisecond)

	mm := getMetricsMap(t, s.URL)
	wts, ok := mm["world_tick_stats"].(map[string]any)
	if !ok || wts == nil {
		t.Fatalf("expected world_tick_stats object, got %#v", mm["world_tick_stats"])
	}
	ws, ok := wts[worldID].(map[string]any)
	if !ok || ws == nil {
		t.Fatalf("expected world entry for %q, got %#v", worldID, wts[worldID])
	}

	tickCount := int64(ws["tick_count_total"].(float64))
	lastMs := int64(ws["tick_last_unix_ms"].(float64))
	if tickCount <= 0 {
		t.Fatalf("expected tick_count_total>0, got %d", tickCount)
	}
	if lastMs <= 0 {
		t.Fatalf("expected tick_last_unix_ms>0, got %d", lastMs)
	}
}

