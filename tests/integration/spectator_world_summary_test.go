package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
}

