package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"strings"

	"lobster-world-core/internal/gateway"
)

func TestSpectatorHome_IncludesWorldStageAndSummary(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_summary"
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "去狩猎获取食物"})
	r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post intents: %v", err)
	}
	r.Body.Close()

	// Give the sim some time to process at least one tick.
	time.Sleep(80 * time.Millisecond)

	resp, err := http.Get(s.URL + "/api/v0/spectator/home?world_id=" + worldID)
	if err != nil {
		t.Fatalf("get home: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	world, ok := out["world"].(map[string]any)
	if !ok {
		t.Fatalf("expected world object, got %#v", out["world"])
	}
	if world["stage"] == "" {
		t.Fatalf("expected world.stage, got %#v", world)
	}
	if _, ok := world["summary"].([]any); !ok {
		t.Fatalf("expected world.summary array, got %#v", world["summary"])
	}
	// New behavior: summary should reflect recent events (intent/shock/evolution).
	var containsHint bool
	var containsHook bool
	var orderCheck []string
	for _, it := range world["summary"].([]any) {
		s, _ := it.(string)
		orderCheck = append(orderCheck, s)
		if strings.Contains(s, "近期") || strings.Contains(s, "刚刚") {
			containsHint = true
		}
		if strings.HasPrefix(s, "看点：") {
			containsHook = true
		}
	}
	if !containsHint {
		t.Fatalf("expected world.summary contains recent-events hint, got %#v", world["summary"])
	}
	if !containsHook {
		t.Fatalf("expected world.summary contains a hook line starting with 看点：, got %#v", world["summary"])
	}

	// Summary order should be stable for clients:
	// 阶段 -> 近期 -> 看点 -> 风险(0..n) -> 建议(0..1)
	pos := func(prefix string) int {
		for i, s := range orderCheck {
			if strings.HasPrefix(s, prefix) {
				return i
			}
		}
		return -1
	}
	pStage := pos("阶段：")
	pRecent := pos("近期：")
	pHook := pos("看点：")
	if pStage < 0 || pRecent < 0 || pHook < 0 {
		t.Fatalf("expected stage/recent/hook lines exist, got %#v", orderCheck)
	}
	if !(pStage < pRecent && pRecent < pHook) {
		t.Fatalf("expected stable order stage<recent<hook, got %#v", orderCheck)
	}
}
