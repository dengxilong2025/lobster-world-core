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

func TestWorldStatus_ReflectsDeltaAfterIntent(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 10 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	worldID := "w_status"

	// Submit intent that should produce a delta (knowledge +1 in v0).
	body, _ := json.Marshal(map[string]any{
		"world_id": worldID,
		"goal":     "研究农业",
	})
	resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Wait a bit for tick to process.
	time.Sleep(50 * time.Millisecond)

	// Query status.
	stResp, err := http.Get(s.URL + "/api/v0/world/status?world_id=" + worldID)
	if err != nil {
		t.Fatalf("GET /world/status: %v", err)
	}
	defer stResp.Body.Close()
	if stResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", stResp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(stResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", out)
	}
	state, ok := out["state"].(map[string]any)
	if !ok {
		t.Fatalf("expected state object, got %#v", out["state"])
	}
	// knowledge should be numeric and >= 1 after delta applied.
	kn, ok := state["knowledge"].(float64)
	if !ok || kn < 1 {
		t.Fatalf("expected knowledge >= 1, got %#v", state["knowledge"])
	}
}

