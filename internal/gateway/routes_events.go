package gateway

import (
	"bufio"
	"encoding/json"
	"net/http"
	"time"

	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/events/stream"
)

func registerEventRoutes(mux *http.ServeMux, es store.EventStore, hub *stream.Hub, mt *Metrics) {
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

		// Subscribe BEFORE sending the first bytes to the client, so callers that connect
		// and immediately post intents don't miss initial events due to a race.
		ch, unsub := hub.Subscribe(256)
		defer unsub()
		start := time.Now()
		if mt != nil {
			mt.IncSSEConnectionsTotal()
			mt.AddSSEConnectionsCurrent(1)
			mt.AddSSEConnectionsCurrentByWorld(worldID, 1)
			defer func() {
				mt.AddSSEConnectionsCurrentByWorld(worldID, -1)
				mt.AddSSEConnectionsCurrent(-1)
				mt.IncSSEDisconnectsTotal()
				mt.ObserveSSEConnDurationMs(time.Since(start).Milliseconds())
			}()
		}

		// Initial comment to establish stream.
		n0, _ := w.Write([]byte(":ok\n\n"))
		if mt != nil && n0 > 0 {
			mt.AddSSEBytes(int64(n0))
		}
		flusher.Flush()

		bw := bufio.NewWriter(w)

		// Replay missed events first (best-effort). This is critical because hub may drop under backpressure.
		if sinceTs > 0 {
			missed, err := es.Query(store.Query{WorldID: worldID, SinceTs: sinceTs, Limit: 500})
			if err == nil {
				for _, e := range missed {
					b, _ := json.Marshal(e)
					n, err := writeSSEMessage(bw, flusher, b)
					if err != nil {
						if mt != nil {
							mt.IncSSEFlushErrorsTotal()
						}
						return
					}
					if mt != nil {
						mt.AddSSEBytes(n)
						mt.IncSSEDataMessagesTotal()
					}
				}
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
				n, err := writeSSEMessage(bw, flusher, b)
				if err != nil {
					if mt != nil {
						mt.IncSSEFlushErrorsTotal()
					}
					return
				}
				if mt != nil {
					mt.AddSSEBytes(n)
					mt.IncSSEDataMessagesTotal()
				}
			}
		}
	})
}
