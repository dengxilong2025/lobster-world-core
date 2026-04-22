package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lobster-world-core/internal/gateway"
)

func TestDebugMetrics_ExposesCounters(t *testing.T) {
	t.Parallel()

	app := gateway.NewApp()
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	// Trigger a couple of requests (401 is fine).
	_, _ = http.Get(s.URL + "/api/v0/me")
	_, _ = http.Get(s.URL + "/api/v0/me")

	resp, err := http.Get(s.URL + "/api/v0/debug/metrics")
	if err != nil {
		t.Fatalf("GET debug/metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	// Very lightweight assertion to keep it stable:
	// we just ensure the JSON contains the keys and request count is present.
	sBody := string(b)
	if !strings.Contains(sBody, "\"requests_total\"") {
		t.Fatalf("expected requests_total, got %s", sBody)
	}
	if !strings.Contains(sBody, "\"responses_by_status\"") {
		t.Fatalf("expected responses_by_status, got %s", sBody)
	}
}

