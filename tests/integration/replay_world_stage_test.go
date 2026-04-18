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
	var recentLine string
	for _, it := range beats {
		m, _ := it.(map[string]any)
		cap, _ := m["caption"].(string)
		if strings.HasPrefix(cap, "近期：") {
			found = true
			recentLine = cap
			break
		}
	}
	if !found {
		t.Fatalf("expected a recent summary beat starting with 近期：, got %#v", beats)
	}
	// For an action_completed replay, "近期" should prefer the meaningful cause (intent_accepted)
	// rather than generic "行动完成" boilerplate.
	if !strings.Contains(recentLine, "意图接受") {
		t.Fatalf("expected recent summary references intent_accepted, got %q", recentLine)
	}
	// Avoid duplicated narratives inside the same 近期：... line.
	// (e.g. "A；A" is noisy to viewers)
	if strings.Contains(recentLine, "；") {
		parts := strings.Split(strings.TrimPrefix(recentLine, "近期："), "；")
		seen := map[string]bool{}
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if seen[p] {
				t.Fatalf("expected recent summary deduped, got %q", recentLine)
			}
			seen[p] = true
		}
	}
}

func TestReplayHighlight_IncludesOneRiskOrAdviceBeat(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_rp_advice"
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
		if strings.HasPrefix(cap, "风险：") || strings.HasPrefix(cap, "建议：") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a risk/advice beat, got %#v", beats)
	}
}

func TestReplayHighlight_IncludesHookBeat(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_rp_hook"
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
		if strings.HasPrefix(cap, "看点：") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a hook beat starting with 看点：, got %#v", beats)
	}
}

func TestReplayHighlight_BeatsHaveStrictlyIncreasingTimeline(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_rp_timeline"
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

	prev := -1
	for i, it := range beats {
		m, _ := it.(map[string]any)
		tv, ok := m["t"].(float64)
		if !ok {
			t.Fatalf("expected numeric t at %d, got %#v", i, m["t"])
		}
		tcur := int(tv)
		if tcur <= prev {
			t.Fatalf("expected strictly increasing t, got %d then %d (index=%d) beats=%#v", prev, tcur, i, beats)
		}
		prev = tcur
	}
}
