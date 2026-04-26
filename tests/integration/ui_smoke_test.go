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

func TestUI_RootRedirectsToUI(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	// Do not follow redirects.
	c := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// 1) No query string.
	resp, err := c.Get(s.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Location"); got != "/ui" {
		t.Fatalf("expected Location=/ui, got %q", got)
	}

	// 2) Preserve query string.
	resp2, err := c.Get(s.URL + "/?world_id=w1&goal=hi")
	if err != nil {
		t.Fatalf("get / with query: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp2.StatusCode)
	}
	if got := resp2.Header.Get("Location"); got != "/ui?world_id=w1&goal=hi" {
		t.Fatalf("expected Location preserves query, got %q", got)
	}
}

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
	// Replay link text should not expose internal event_id; keep it in data-* for tooling.
	if !strings.Contains(body, "dataset.eventId") {
		t.Fatalf("expected /ui stores event_id in dataset for replay links")
	}
	// /ui should support query params for agentic usage (world_id/goal) and optional autoconnect.
	// We assert code presence rather than full browser execution.
	if !strings.Contains(body, "URLSearchParams") {
		t.Fatalf("expected /ui parses URLSearchParams for scriptable params")
	}
	if !strings.Contains(body, "autoconnect") {
		t.Fatalf("expected /ui supports autoconnect param")
	}
	// UX: when no world_id param is provided, UI should auto connect to default world.
	if !strings.Contains(body, "if (!wid)") {
		t.Fatalf("expected /ui autoconnects when no world_id provided")
	}

	// Export entrypoint should be discoverable from UI (v0.2 requirement).
	if !strings.Contains(body, "/api/v0/replay/export") {
		t.Fatalf("expected page references replay/export endpoint")
	}
	if !strings.Contains(body, "id=\"btn_export\"") {
		t.Fatalf("expected export button id=btn_export")
	}
}
