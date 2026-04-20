package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func TestUIAssets_ServesHTML(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	resp, err := http.Get(s.URL + "/ui/assets")
	if err != nil {
		t.Fatalf("get /ui/assets: %v", err)
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
	if !strings.Contains(body, "/assets/production/manifest.json") {
		t.Fatalf("expected page references manifest.json")
	}
	if !strings.Contains(body, "id=\"asset_modal\"") {
		t.Fatalf("expected assets page contains #asset_modal")
	}
	if !strings.Contains(body, "id=\"canvas_3x3\"") {
		t.Fatalf("expected assets page contains #canvas_3x3")
	}
	if !strings.Contains(body, "id=\"btn_export_3x3\"") {
		t.Fatalf("expected assets page contains #btn_export_3x3")
	}
	if !strings.Contains(body, "id=\"export_log_panel\"") {
		t.Fatalf("expected assets page contains #export_log_panel")
	}
	if !strings.Contains(body, "id=\"btn_copy_qc\"") {
		t.Fatalf("expected assets page contains #btn_copy_qc")
	}
	if !strings.Contains(body, "id=\"btn_qa\"") {
		t.Fatalf("expected assets page contains #btn_qa")
	}
}

func TestAssetManifest_Served(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	resp, err := http.Get(s.URL + "/assets/production/manifest.json")
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if len(b) < 10 {
		t.Fatalf("manifest too small")
	}
	// Very light assertion: should be JSON-like.
	if b[0] != '{' && b[0] != '[' {
		t.Fatalf("manifest does not look like json (first byte=%q)", b[0])
	}
}

func TestAssetManifest_FileExistsInRepo(t *testing.T) {
	t.Parallel()

	// `go test` runs each package with its own working directory (often the package dir),
	// so we probe a few repo-relative locations rather than assuming repo root.
	paths := []string{
		"assets/production/manifest.json",
		"../assets/production/manifest.json",
		"../../assets/production/manifest.json",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return
		}
	}

	wd, _ := os.Getwd()
	t.Fatalf("expected assets/production/manifest.json present in repo (wd=%q, tried=%v)", wd, paths)
}
