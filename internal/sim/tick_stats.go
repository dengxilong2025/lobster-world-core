package sim

// TickStat is a low-cardinality, wall-clock-based snapshot for observability only.
// It must not influence simulation decisions or event timelines.
type TickStat struct {
	TickCountTotal    int64 `json:"tick_count_total"`
	TickLastUnixMs    int64 `json:"tick_last_unix_ms"`
	TickJitterMsTotal int64 `json:"tick_jitter_ms_total"`
	TickJitterCount   int64 `json:"tick_jitter_count"`
	TickOverrunTotal  int64 `json:"tick_overrun_total"`
}

// TickStats returns per-world tick wall-clock statistics for debug/metrics.
func (e *Engine) TickStats() map[string]TickStat {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := map[string]TickStat{}
	for id, w := range e.worlds {
		if w == nil {
			continue
		}
		out[id] = w.tickStats()
	}
	return out
}

