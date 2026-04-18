package gateway

import (
	"bufio"
	"encoding/json"
	"net/http"

	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
)

func registerEventRoutes(mux *http.ServeMux, es store.EventStore, hub *stream.Hub) {
	mux.HandleFunc("GET /api/v0/events", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		worldID := q.Get("world_id")
		sinceTs := parseInt64(q.Get("since_ts"))
		limit := parseInt(q.Get("limit"))
		entityID := q.Get("entity_id")

		events, err := es.Query(store.Query{
			WorldID:  worldID,
			SinceTs:  sinceTs,
			Limit:    limit,
			EntityID: entityID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"events": events,
		})
	})

	// SSE event stream. Transport is decoupled from the event object.
	mux.HandleFunc("GET /api/v0/events/stream", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		worldID := q.Get("world_id")
		if worldID == "" {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "world_id is required")
			return
		}
		sinceTs := parseInt64(q.Get("since_ts"))

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "streaming unsupported")
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Initial comment to establish stream.
		_, _ = w.Write([]byte(":ok\n\n"))
		flusher.Flush()

		ch, unsub := hub.Subscribe(256)
		defer unsub()

		bw := bufio.NewWriter(w)

		// Replay missed events first (best-effort). This is critical because hub may drop under backpressure.
		if sinceTs > 0 {
			missed, err := es.Query(store.Query{WorldID: worldID, SinceTs: sinceTs, Limit: 500})
			if err == nil {
				for _, e := range missed {
					b, _ := json.Marshal(e)
					_, _ = bw.WriteString("event: message\n")
					_, _ = bw.WriteString("data: ")
					_, _ = bw.Write(b)
					_, _ = bw.WriteString("\n\n")
				}
				_ = bw.Flush()
				flusher.Flush()
			}
		}

		for {
			select {
			case <-r.Context().Done():
				return
			case e, ok := <-ch:
				if !ok {
					return
				}
				if e.WorldID != worldID {
					continue
				}
				b, _ := json.Marshal(e)
				_, _ = bw.WriteString("event: message\n")
				_, _ = bw.WriteString("data: ")
				_, _ = bw.Write(b)
				_, _ = bw.WriteString("\n\n")
				_ = bw.Flush()
				flusher.Flush()
			}
		}
	})
}

