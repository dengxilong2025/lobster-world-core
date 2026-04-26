package gateway

import "net/http"

func registerUIRoutes(mux *http.ServeMux) {
	// Root entrypoint: redirect to /ui (keep query string) to avoid staging confusion.
	// NOTE: Use path-only pattern (no method) to avoid ServeMux pattern conflicts with other
	// method-agnostic handlers like /assets/production/.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		target := "/ui"
		if r.URL.RawQuery != "" {
			target = target + "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusFound) // 302
	})

	mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(uiPageHTML))
	})
}
