package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/gateway"
)

func TestIntents_AreProcessedByTickSimWithMonotonicTick(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 10 * time.Millisecond,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	worldID := "w_test"

	// Connect SSE first.
	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/v0/events/stream?world_id="+worldID, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("connect stream: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	br := bufio.NewReader(resp.Body)

	// Submit intent.
	body, _ := json.Marshal(map[string]any{
		"world_id": worldID,
		"goal":     "优先贸易",
	})
	intentResp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	intentResp.Body.Close()
	if intentResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", intentResp.StatusCode)
	}

	// Read three events: intent_accepted, action_started, action_completed.
	e1 := readNextDataEventLocal(t, br, 1500*time.Millisecond)
	e2 := readNextDataEventLocal(t, br, 1500*time.Millisecond)
	e3 := readNextDataEventLocal(t, br, 1500*time.Millisecond)

	if e1.Type != "intent_accepted" {
		t.Fatalf("expected intent_accepted, got %s", e1.Type)
	}
	if e2.Type != "action_started" {
		t.Fatalf("expected action_started, got %s", e2.Type)
	}
	if e3.Type != "action_completed" {
		t.Fatalf("expected action_completed, got %s", e3.Type)
	}
	if !(e1.Tick <= e2.Tick && e2.Tick == e3.Tick) {
		t.Fatalf("unexpected tick order: %d, %d, %d", e1.Tick, e2.Tick, e3.Tick)
	}
}

func readNextDataEventLocal(t *testing.T, br *bufio.Reader, timeout time.Duration) spec.Event {
	t.Helper()

	deadline := time.Now().Add(timeout)
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
			return e
		}
	}
	t.Fatalf("timed out waiting for data line")
	return spec.Event{}
}

