package stream

import (
	"sync"

	"lobster-world-core/internal/events/spec"
)

// Hub is a minimal in-process pub/sub for events.
//
// v0 scope:
// - Fan-out events to connected SSE clients.
// - Used by API handlers and (later) the simulation core.
//
// Note: This is not durable. Durability comes from EventStore (append-only log).
type Hub struct {
	mu   sync.RWMutex
	subs map[chan spec.Event]struct{}
}

func NewHub() *Hub {
	return &Hub{
		subs: map[chan spec.Event]struct{}{},
	}
}

// Subscribe registers a new subscriber channel.
// Caller MUST call the returned unsubscribe function.
func (h *Hub) Subscribe(buffer int) (<-chan spec.Event, func()) {
	if buffer <= 0 {
		buffer = 64
	}
	ch := make(chan spec.Event, buffer)

	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
		h.mu.Unlock()
	}

	return ch, unsub
}

func (h *Hub) Publish(e spec.Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subs {
		// Non-blocking to avoid slow consumers freezing the producer.
		select {
		case ch <- e:
		default:
		}
	}
}

