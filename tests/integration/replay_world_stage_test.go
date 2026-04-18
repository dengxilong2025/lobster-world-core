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

func TestReplayHighlight_IncludesWorldStageBeat(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_rp_stage"
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "去狩猎获取食物"})
	resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()

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

	var out map[string]any
	if err := json.NewDecoder(rp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	beats, ok := out["beats"].([]any)
	if !ok || len(beats) == 0 {
		t.Fatalf("expected beats, got %#v", out["beats"])
	}
	found := false
	for _, it := range beats {
		m, _ := it.(map[string]any)
		cap, _ := m["caption"].(string)
		if strings.HasPrefix(cap, "世界阶段：") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a world stage beat, got %#v", beats)
	}
}

func TestReplayHighlight_IncludesRecentSummaryBeat(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_rp_summary"
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "去狩猎获取食物"})
	resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	resp.Body.Close()

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

	var out map[string]any
	if err := json.NewDecoder(rp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	beats, ok := out["beats"].([]any)
	if !ok || len(beats) == 0 {
		t.Fatalf("expected beats, got %#v", out["beats"])
	}
	found := false
	for _, it := range beats {
		m, _ := it.(map[string]any)
		cap, _ := m["caption"].(string)
		if strings.HasPrefix(cap, "近期：") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a recent summary beat starting with 近期：, got %#v", beats)
	}
}
