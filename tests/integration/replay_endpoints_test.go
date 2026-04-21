package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func TestReplayHighlight_MainChain_ReturnsBeats(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	// Submit intent to ensure at least one meaningful event exists.
	body, _ := json.Marshal(map[string]any{"world_id": "w1", "goal": "去狩猎获取食物"})
	resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post intent: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Pull events and pick the latest one (intent_accepted should exist).
	evResp, err := http.Get(s.URL + "/api/v0/events?world_id=w1&since_ts=0&limit=10")
	if err != nil {
		t.Fatalf("get events: %v", err)
	}
	defer evResp.Body.Close()
	var evBody struct {
		OK     bool            `json:"ok"`
		Events []map[string]any `json:"events"`
	}
	if err := json.NewDecoder(evResp.Body).Decode(&evBody); err != nil {
		t.Fatalf("decode events: %v", err)
	}
	if len(evBody.Events) == 0 {
		t.Fatalf("expected at least 1 event")
	}
	eventID, _ := evBody.Events[len(evBody.Events)-1]["event_id"].(string)
	if eventID == "" {
		t.Fatalf("expected event_id in events payload")
	}

	hl, err := http.Get(s.URL + "/api/v0/replay/highlight?world_id=w1&event_id=" + eventID)
	if err != nil {
		t.Fatalf("get highlight: %v", err)
	}
	defer hl.Body.Close()
	if hl.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(hl.Body)
		t.Fatalf("expected 200, got %d body=%s", hl.StatusCode, string(b))
	}
	var out struct {
		OK       bool              `json:"ok"`
		ReplayID string            `json:"replay_id"`
		EventID  string            `json:"event_id"`
		Beats    []map[string]any  `json:"beats"`
	}
	if err := json.NewDecoder(hl.Body).Decode(&out); err != nil {
		t.Fatalf("decode highlight: %v", err)
	}
	if !out.OK || out.ReplayID == "" || out.EventID == "" {
		t.Fatalf("unexpected highlight envelope: ok=%v replay_id=%q event_id=%q", out.OK, out.ReplayID, out.EventID)
	}
	if len(out.Beats) == 0 {
		t.Fatalf("expected beats non-empty")
	}
	// beats should have captions
	c0, _ := out.Beats[0]["caption"].(string)
	if strings.TrimSpace(c0) == "" {
		t.Fatalf("expected first beat has caption")
	}
}

// Note: replay/export NDJSON correctness + determinism is already covered by
// tests/integration/replay_export_test.go. This file focuses on a "main chain"
// flow where we generate events via /intents then open replay/highlight.
