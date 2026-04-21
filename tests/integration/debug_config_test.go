package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func TestDebugConfig_ReturnsSafeRuntimeConfig(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval:        20 * time.Millisecond,
		TrustedProxyCIDRs:   []string{"10.0.0.0/8"},
		IntentAcceptTimeout: 123 * time.Millisecond,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	resp, err := http.Get(s.URL + "/api/v0/debug/config")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", out)
	}
	cfg, _ := out["config"].(map[string]any)
	if cfg == nil {
		t.Fatalf("expected config object, got %#v", out)
	}
	if cfg["intent_accept_timeout_ms"] != float64(123) {
		t.Fatalf("expected intent_accept_timeout_ms=123, got %#v", cfg["intent_accept_timeout_ms"])
	}
	// Safe: only CIDRs, no secrets.
	if _, ok := cfg["trusted_proxy_cidrs"]; !ok {
		t.Fatalf("expected trusted_proxy_cidrs field")
	}
	if _, ok := cfg["shock_enabled"]; !ok {
		t.Fatalf("expected shock_enabled field")
	}
}

