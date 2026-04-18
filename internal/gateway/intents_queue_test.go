package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIntents_Returns503WhenSimQueueIsFull(t *testing.T) {
	t.Parallel()

	app := NewAppWithOptions(AppOptions{
		TickInterval:   5 * time.Second, // keep queue from draining
		MaxIntentQueue: 1,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_busy"
	body1, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "a"})
	resp1, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body1))
	if err != nil {
		t.Fatalf("post1: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}

	body2, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "b"})
	resp2, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body2))
	if err != nil {
		t.Fatalf("post2: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp2.StatusCode)
	}
}

