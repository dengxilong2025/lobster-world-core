package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func TestUI_ServesHTML(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	resp, err := http.Get(s.URL + "/ui")
	if err != nil {
		t.Fatalf("get /ui: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	body := string(b)

	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		t.Fatalf("expected text/html content-type, got %q", resp.Header.Get("Content-Type"))
	}
	if !strings.Contains(body, "id=\"world_id\"") {
		head := body
		if len(head) > 200 {
			head = head[:200]
		}
		t.Fatalf("expected #world_id input, got body head: %q", head)
	}
	if !strings.Contains(body, "/api/v0/intents") || !strings.Contains(body, "/api/v0/events/stream") {
		t.Fatalf("expected page references v0 api endpoints")
	}
	// Ensure key DOM ids exist for agentic testers.
	for _, id := range []string{"btn_intent", "btn_connect", "events", "world_stage", "world_summary"} {
		if !strings.Contains(body, "id=\""+id+"\"") {
			t.Fatalf("expected element id=%q", id)
		}
	}
	// Replay entrypoint should be discoverable from UI.
	if !strings.Contains(body, "/api/v0/replay/highlight") {
		t.Fatalf("expected page references replay/highlight endpoint")
	}
	// /ui should support query params for agentic usage (world_id/goal) and optional autoconnect.
	// We assert code presence rather than full browser execution.
	if !strings.Contains(body, "URLSearchParams") {
		t.Fatalf("expected /ui parses URLSearchParams for scriptable params")
	}
	if !strings.Contains(body, "autoconnect") {
		t.Fatalf("expected /ui supports autoconnect param")
	}

	// Export entrypoint should be discoverable from UI (v0.2 requirement).
	if !strings.Contains(body, "/api/v0/replay/export") {
		t.Fatalf("expected page references replay/export endpoint")
	}
	if !strings.Contains(body, "id=\"btn_export\"") {
		t.Fatalf("expected export button id=btn_export")
	}
}
