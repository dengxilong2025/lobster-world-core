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

func TestSpectatorHome_IsRealtimeAfterFirstLoad(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_rt"

	// First call populates projection cache.
	resp1, err := http.Get(s.URL + "/api/v0/spectator/home?world_id=" + worldID)
	if err != nil {
		t.Fatalf("GET home: %v", err)
	}
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}

	// Now produce new events.
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
	resp2, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST intents: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	// Poll until spectator sees the new event (without reloading from store).
	deadline := time.Now().Add(600 * time.Millisecond)
	for time.Now().Before(deadline) {
		resp, err := http.Get(s.URL + "/api/v0/spectator/home?world_id=" + worldID)
		if err != nil {
			t.Fatalf("GET home2: %v", err)
		}
		var out struct {
			OK       bool `json:"ok"`
			Headline struct {
				Title string `json:"title"`
			} `json:"headline"`
			HotEvents []struct {
				Title string `json:"title"`
			} `json:"hot_events"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if out.Headline.Title != "" && strings.Contains(out.Headline.Title, "意图接受") {
			return
		}
		for _, e := range out.HotEvents {
			if strings.Contains(e.Title, "意图接受") {
				return
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for spectator realtime update")
}
