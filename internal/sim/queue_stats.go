package sim

// QueueStat is a low-cardinality, debug-oriented snapshot of per-world backpressure.
// It is safe to expose via debug/metrics as it contains no event content, only sizes.
type QueueStat struct {
	IntentChLen     int   `json:"intent_ch_len"`
	IntentChCap     int   `json:"intent_ch_cap"`
	PendingQueueLen int   `json:"pending_queue_len"`
	PendingQueueMax int   `json:"pending_queue_max"`
	Tick            int64 `json:"tick"`
}

// QueueStats returns a snapshot of per-world queue depths.
// This is for observability only and must not affect sim determinism.
func (e *Engine) QueueStats() map[string]QueueStat {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := map[string]QueueStat{}
	for id, w := range e.worlds {
		if w == nil {
			continue
		}
		out[id] = w.queueStats()
	}
	return out
}

