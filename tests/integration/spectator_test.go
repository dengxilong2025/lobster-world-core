package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/gateway"
)

func TestSpectatorHome_ReturnsHeadlineAndHotEvents(t *testing.T) {
	t.Parallel()

	app := gateway.NewApp()
	_ = app.EventStore.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_1",
		Ts:            10,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "shock_warning",
		Actors:        []string{"lobster_1"},
		Narrative:     "天象异常：裂冬指数上升",
	})
	_ = app.EventStore.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_2",
		Ts:            20,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "shock_started",
		Actors:        []string{"lobster_1"},
		Narrative:     "冲击开始：裂冬纪元降临",
	})

	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	resp, err := http.Get(s.URL + "/api/v0/spectator/home?world_id=w1")
	if err != nil {
		t.Fatalf("GET /spectator/home: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", out)
	}
	headline, ok := out["headline"].(map[string]any)
	if !ok || headline["event_id"] == "" {
		t.Fatalf("expected headline with event_id, got %#v", out["headline"])
	}
	hot, ok := out["hot_events"].([]any)
	if !ok || len(hot) < 2 {
		t.Fatalf("expected >=2 hot_events, got %#v", out["hot_events"])
	}
}

