package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/gateway"
)

func TestReplayHighlight_ReturnsBeats(t *testing.T) {
	t.Parallel()

	app := gateway.NewApp()
	_ = app.EventStore.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_replay_1",
		Ts:            10,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "betrayal",
		Actors:        []string{"a"},
		Narrative:     "盟友反咬：背叛写进史册",
		Trace: []spec.TraceLink{
			{CauseEventID: "evt_prev", Note: "裂冬预兆导致粮荒"},
			{CauseEventID: "evt_mid", Note: "盟誓条款争议扩大"},
		},
	})

	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	resp, err := http.Get(s.URL + "/api/v0/replay/highlight?world_id=w1&event_id=evt_replay_1")
	if err != nil {
		t.Fatalf("GET replay: %v", err)
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
	if out["replay_id"] == "" {
		t.Fatalf("expected replay_id, got %#v", out)
	}
	beats, ok := out["beats"].([]any)
	if !ok || len(beats) == 0 {
		t.Fatalf("expected non-empty beats, got %#v", out["beats"])
	}
	first, _ := beats[0].(map[string]any)
	if first["caption"] != "盟友反咬：背叛写进史册" {
		t.Fatalf("expected first beat caption to match narrative, got %#v", first)
	}
	second, _ := beats[1].(map[string]any)
	if second["caption"] != "因为：裂冬预兆导致粮荒" {
		t.Fatalf("expected second beat caption from trace, got %#v", second)
	}
}
