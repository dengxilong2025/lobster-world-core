package gateway

import (
	"net/http"
	"os"
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
	// Prefer repo-relative path when running locally.
	for _, p := range []string{
		"assets/production",
		"../assets/production",
		"/assets/production", // useful in containers if we copy assets to /assets/production
	} {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}

	// Fall back to repo-relative; FileServer will 404 if missing.
	return "assets/production"
}
