package integration

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/gateway"
)

func TestEvents_StreamSinceTs_ReplaysMissedEvents(t *testing.T) {
	t.Parallel()

	app := gateway.NewApp()

	// Preload one event at ts=10.
	_ = app.EventStore.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_old",
		Ts:            10,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "shock_warning",
		Actors:        []string{"a"},
		Narrative:     "n-old",
	})

	// Append another event at ts=20 that should be replayed when since_ts=10.
	_ = app.EventStore.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_new",
		Ts:            20,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "shock_started",
		Actors:        []string{"a"},
		Narrative:     "n-new",
	})

	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/v0/events/stream?world_id=w1&since_ts=10", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("connect stream: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	br := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			var e spec.Event
			if err := json.Unmarshal([]byte(payload), &e); err != nil {
				t.Fatalf("unmarshal event: %v payload=%q", err, payload)
			}
			// The first replayed event should be evt_new (ts=20).
			if e.EventID != "evt_new" {
				t.Fatalf("expected evt_new, got %s", e.EventID)
			}
			return
		}
	}
	t.Fatalf("timed out waiting for replayed data")
}

