package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

// This is a true "main chain" smoke test:
// - Open SSE stream
// - Submit an intent
// - Assert SSE receives an event
func TestUI_MainChain_SSEReceivesEvent(t *testing.T) {
	// Avoid t.Parallel(): this test is timing sensitive and spawns a streaming connection.

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1) Open SSE stream (world w1).
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+"/api/v0/events/stream?world_id=w1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open sse: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// 2) Submit intent after the stream is established (the stream sends an initial comment).
	body, _ := json.Marshal(map[string]any{"world_id": "w1", "goal": "去狩猎获取食物"})
	postDone := make(chan error, 1)
	go func() {
		// small delay to ensure client is reading
		time.Sleep(50 * time.Millisecond)
		r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
		if err != nil {
			postDone <- err
			return
		}
		_ = r.Body.Close()
		if r.StatusCode != http.StatusOK {
			postDone <- err
			return
		}
		postDone <- nil
	}()

	// 3) Read SSE lines until we get a "data:" payload for intent_accepted.
	type event struct {
		Type    string `json:"type"`
		WorldID string `json:"world_id"`
		EventID string `json:"event_id"`
	}

	linesCh := make(chan string, 32)
	readErrCh := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			linesCh <- sc.Text()
		}
		readErrCh <- sc.Err()
	}()

	posted := false
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for sse event")
		case err := <-postDone:
			if err != nil {
				t.Fatalf("intent post failed: %v", err)
			}
			posted = true
		case err := <-readErrCh:
			if err != nil {
				t.Fatalf("read sse: %v", err)
			}
			t.Fatalf("sse stream ended unexpectedly")
		case line := <-linesCh:
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data: ") {
				payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
				var ev event
				if err := json.Unmarshal([]byte(payload), &ev); err != nil {
					continue
				}
				if ev.WorldID == "w1" && ev.Type == "intent_accepted" && ev.EventID != "" {
					if !posted {
						// Ensure POST has completed (best-effort).
						select {
						case err := <-postDone:
							if err != nil {
								t.Fatalf("intent post failed: %v", err)
							}
						case <-time.After(200 * time.Millisecond):
						}
					}
					return
				}
			}
		}
	}
}
