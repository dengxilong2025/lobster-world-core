package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func TestReplayHighlight_UsesTraceNotesAndCauseNarratives(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_rp_trace"
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "去狩猎获取食物"})
	resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	events := waitEvents(t, app.EventStore, worldID, 3, 1200*time.Millisecond)
	var targetID string
	for _, e := range events {
		if e.Type == "action_completed" {
			targetID = e.EventID
			break
		}
	}
	if targetID == "" {
		t.Fatalf("expected action_completed")
	}

	rp, err := http.Get(s.URL + "/api/v0/replay/highlight?world_id=" + worldID + "&event_id=" + targetID)
	if err != nil {
		t.Fatalf("get replay: %v", err)
	}
	defer rp.Body.Close()
	if rp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", rp.StatusCode)
	}

	var out struct {
		OK    bool `json:"ok"`
		Beats []struct {
			T       int    `json:"t"`
			Caption string `json:"caption"`
		} `json:"beats"`
	}
	if err := json.NewDecoder(rp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK || len(out.Beats) == 0 {
		t.Fatalf("expected beats, got %#v", out)
	}

	// Should include trace-derived "规则解释" and reference the cause intent acceptance narrative.
	var hasRule, hasIntentAccepted bool
	for _, b := range out.Beats {
		if strings.Contains(b.Caption, "规则解释") {
			hasRule = true
		}
		if strings.Contains(b.Caption, "意图接受") {
			hasIntentAccepted = true
		}
	}
	if !hasRule {
		t.Fatalf("expected beats include rule explanation, got %#v", out.Beats)
	}
	if !hasIntentAccepted {
		t.Fatalf("expected beats include cause narrative intent_accepted, got %#v", out.Beats)
	}
}
