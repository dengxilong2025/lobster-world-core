package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"lobster-world-core/internal/gateway"
)

func TestHealthz_ReturnsOK(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(gateway.NewHandler())
	t.Cleanup(s.Close)

	resp, err := http.Get(s.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(b) != "ok" {
		t.Fatalf("expected body 'ok', got %q", string(b))
	}
}

