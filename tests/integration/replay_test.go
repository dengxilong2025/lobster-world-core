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
}

