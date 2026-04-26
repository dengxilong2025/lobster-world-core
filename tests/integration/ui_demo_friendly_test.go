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

func TestUI_IncludesDemoFriendlyBlocks(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	resp, err := http.Get(s.URL + "/ui")
	if err != nil {
		t.Fatalf("GET /ui: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	html := string(b)

	// Ensure debug endpoints are referenced.
	if !strings.Contains(html, "/api/v0/debug/build") {
		t.Fatalf("ui should reference /api/v0/debug/build")
	}
	if !strings.Contains(html, "/api/v0/debug/metrics") {
		t.Fatalf("ui should reference /api/v0/debug/metrics")
	}

	// Ensure key event types appear in highlight mapping.
	for _, typ := range []string{
		"betrayal",
		"war_started",
		"market_boom",
		"trade_dispute",
	} {
		if !strings.Contains(html, typ) {
			t.Fatalf("ui should reference event type %q for highlighting", typ)
		}
	}

	// Ensure trade demo buttons exist.
	for _, id := range []string{
		"btn_demo_boom",
		"btn_demo_dispute",
	} {
		if !strings.Contains(html, id) {
			t.Fatalf("ui should include demo button id %q", id)
		}
	}
}
