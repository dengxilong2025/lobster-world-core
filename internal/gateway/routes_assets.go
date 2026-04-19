package gateway

import (
	"net/http"
	"os"
	"path/filepath"
)

func registerAssetRoutes(mux *http.ServeMux) {
	dir := assetProductionDir()
	fs := http.FileServer(http.Dir(dir))

	// Serve files under assets/production at a stable URL.
	mux.Handle("/assets/production/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		http.StripPrefix("/assets/production/", fs).ServeHTTP(w, r)
	}))

	mux.HandleFunc("/ui/assets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(uiAssetsPageHTML))
	})
}

func assetProductionDir() string {
	// Useful in containers if we copy assets to /assets/production.
	if st, err := os.Stat("/assets/production"); err == nil && st.IsDir() {
		return "/assets/production"
	}

	// In Go tests, the working directory is typically the package directory
	// (e.g. tests/integration or internal/gateway), so "assets/production" is
	// not directly reachable. Search upward from cwd to find the repo root.
	if wd, err := os.Getwd(); err == nil {
		cur := wd
		for i := 0; i < 10; i++ {
			cand := filepath.Join(cur, "assets", "production")
			if st, err := os.Stat(cand); err == nil && st.IsDir() {
				return cand
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}

	// Fall back to repo-relative; FileServer will 404 if missing.
	return filepath.Join("assets", "production")
}
