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

func TestEvents_TailReturnsAppendedEvents(t *testing.T) {
	t.Parallel()

	app := gateway.NewApp()

	_ = app.EventStore.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_1",
		Ts:            10,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "shock_warning",
		Actors:        []string{"lobster_1"},
		Narrative:     "n1",
	})
	_ = app.EventStore.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_2",
		Ts:            20,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "shock_started",
		Actors:        []string{"lobster_1"},
		Narrative:     "n2",
	})

	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	resp, err := http.Get(s.URL + "/api/v0/events?world_id=w1&since_ts=0&limit=10")
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out struct {
		OK     bool         `json:"ok"`
		Events []spec.Event `json:"events"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK {
		t.Fatalf("expected ok=true")
	}
	if len(out.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(out.Events))
	}
}

func TestEvents_StreamSendsPublishedEvent(t *testing.T) {
	t.Parallel()

	app := gateway.NewApp()
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/v0/events/stream?world_id=w1", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("connect stream: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Publish after the stream is established.
	go func() {
		time.Sleep(50 * time.Millisecond)
		app.Hub.Publish(spec.Event{
			SchemaVersion: 1,
			EventID:       "evt_pub",
			Ts:            30,
			WorldID:       "w1",
			Scope:         "world",
			Type:          "shock_warning",
			Actors:        []string{"lobster_1"},
			Narrative:     "hello",
		})
	}()

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
			if e.EventID != "evt_pub" {
				t.Fatalf("expected evt_pub, got %s", e.EventID)
			}
			return
		}
	}
	t.Fatalf("timed out waiting for data line")
}

