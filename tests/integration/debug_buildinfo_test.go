package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
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

	// Required: git_sha must always be non-empty (best-effort real sha, fallback "unknown").
	v, ok := out.Build["git_sha"]
	if !ok {
		t.Fatalf("missing git_sha")
	}
	gitSHA, _ := v.(string)
	if gitSHA == "" {
		t.Fatalf("git_sha empty")
	}
}

func TestDebugBuild_GitHubFallback_WhenUnknown(t *testing.T) {
	t.Parallel()

	// Fake GitHub API server.
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"sha":"0123456789abcdef0123456789abcdef01234567"}`))
	}))
	t.Cleanup(gh.Close)

	ghResolver := gateway.NewGitHubCommitResolver(gateway.GitHubCommitResolverOptions{
		BaseURL: gh.URL,
	})

	// Simulate an environment where buildvcs is unavailable (Render/Docker runtime without VCS metadata),
	// so git_sha becomes "unknown" and must fall back to GitHub API.
	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 20 * time.Millisecond,
		ReadBuildInfo: func() (*debug.BuildInfo, bool) { return nil, false },
		GitHubCommitResolver: ghResolver,
	})
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
	if got, _ := out.Build["git_sha"].(string); got != "0123456" {
		t.Fatalf("expected sha7 from github fallback, got=%v", out.Build["git_sha"])
	}
}
