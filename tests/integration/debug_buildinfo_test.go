package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func TestDebugBuild_ReturnsBuildInfo(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	resp, err := http.Get(s.URL + "/api/v0/debug/build")
	if err != nil {
		t.Fatalf("GET debug/build: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out struct {
		OK    bool                   `json:"ok"`
		Build map[string]interface{} `json:"build"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK {
		t.Fatalf("expected ok=true")
	}
	if out.Build == nil {
		t.Fatalf("expected build object")
	}

	// Required fields
	if _, ok := out.Build["start_time"]; !ok {
		t.Fatalf("missing start_time")
	}
	if _, ok := out.Build["uptime_sec"]; !ok {
		t.Fatalf("missing uptime_sec")
	}

	// Optional but useful (may be empty depending on build flags)
	if v, ok := out.Build["git_sha"]; ok {
		if s, ok := v.(string); ok && s == "" {
			t.Fatalf("git_sha present but empty")
		}
	}
}

