package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lobster-world-core/internal/auth"
)

func TestAuthEndpoints_AreRateLimitedByIP(t *testing.T) {
	t.Parallel()

	// Use a real auth service; we only test rate limiting, not auth correctness.
	h := NewHandler(Options{Auth: auth.NewService(auth.Options{})})
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)

	body, _ := json.Marshal(map[string]any{"lobster_pubkey": "bad", "client_ts": 1})

	// Burst: first two should pass (400 is ok - it means handler ran),
	// third should be rate-limited (429).
	for i := 0; i < 2; i++ {
		resp, err := http.Post(s.URL+"/api/v0/auth/challenge", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("unexpected 429 on request %d", i+1)
		}
	}

	resp, err := http.Post(s.URL+"/api/v0/auth/challenge", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post3: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
}

