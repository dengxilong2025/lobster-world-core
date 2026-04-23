package gateway

import (
	"sync"
	"sync/atomic"
)

// Metrics is a tiny in-process counters set for debugging/ops.
// It intentionally avoids high-cardinality labels and third-party dependencies.
type Metrics struct {
	requestsTotal atomic.Int64
	busyTotal     atomic.Int64

	// EventStore write health (Append only).
	eventstoreAppendTotal       atomic.Int64
	eventstoreAppendErrorsTotal atomic.Int64

	// Replay export/highlight.
	replayExportTotal      atomic.Int64
	replayExportErrors     atomic.Int64
	replayExportTimeMs     atomic.Int64
	replayExportBytesTotal atomic.Int64

	replayHighlightTotal  atomic.Int64
	replayHighlightErrors atomic.Int64
	replayHighlightTimeMs atomic.Int64

	// SSE /events/stream
	sseConnectionsCurrent atomic.Int64
	sseConnectionsTotal   atomic.Int64
	sseDisconnectsTotal   atomic.Int64
	sseDataMessagesTotal  atomic.Int64

	mu       sync.Mutex
	byStatus map[int]*atomic.Int64
}

func NewMetrics() *Metrics {
	return &Metrics{
		byStatus: map[int]*atomic.Int64{},
	}
}

func (m *Metrics) IncRequest() {
	m.requestsTotal.Add(1)
}

func (m *Metrics) IncStatus(code int) {
	m.mu.Lock()
	c, ok := m.byStatus[code]
	if !ok {
		c = &atomic.Int64{}
		m.byStatus[code] = c
	}
	m.mu.Unlock()
	c.Add(1)
}

func (m *Metrics) IncBusy() {
	m.busyTotal.Add(1)
}

func (m *Metrics) IncEventStoreAppend() {
	m.eventstoreAppendTotal.Add(1)
}

func (m *Metrics) IncEventStoreAppendError() {
	m.eventstoreAppendErrorsTotal.Add(1)
}

func (m *Metrics) IncReplayExport() {
	m.replayExportTotal.Add(1)
}

func (m *Metrics) IncReplayExportError() {
	m.replayExportErrors.Add(1)
}

func (m *Metrics) AddReplayExportTimeMs(ms int64) {
	if ms > 0 {
		m.replayExportTimeMs.Add(ms)
	}
}

func (m *Metrics) AddReplayExportBytes(n int64) {
	if n > 0 {
		m.replayExportBytesTotal.Add(n)
	}
}

func (m *Metrics) IncReplayHighlight() {
	m.replayHighlightTotal.Add(1)
}

func (m *Metrics) IncReplayHighlightError() {
	m.replayHighlightErrors.Add(1)
}

func (m *Metrics) AddReplayHighlightTimeMs(ms int64) {
	if ms > 0 {
		m.replayHighlightTimeMs.Add(ms)
	}
}

func (m *Metrics) AddSSEConnectionsCurrent(delta int64) {
	m.sseConnectionsCurrent.Add(delta)
}

func (m *Metrics) IncSSEConnectionsTotal() {
	m.sseConnectionsTotal.Add(1)
}

func (m *Metrics) IncSSEDisconnectsTotal() {
	m.sseDisconnectsTotal.Add(1)
}

func (m *Metrics) IncSSEDataMessagesTotal() {
	m.sseDataMessagesTotal.Add(1)
}

func (m *Metrics) Snapshot() map[string]any {
	out := map[string]any{
		"requests_total": m.requestsTotal.Load(),
		"busy_total":     m.busyTotal.Load(),
		"eventstore_append_total":        m.eventstoreAppendTotal.Load(),
		"eventstore_append_errors_total": m.eventstoreAppendErrorsTotal.Load(),
		"replay_export_total":            m.replayExportTotal.Load(),
		"replay_export_errors_total":     m.replayExportErrors.Load(),
		"replay_export_time_ms_total":    m.replayExportTimeMs.Load(),
		"replay_export_bytes_total":      m.replayExportBytesTotal.Load(),
		"replay_highlight_total":         m.replayHighlightTotal.Load(),
		"replay_highlight_errors_total":  m.replayHighlightErrors.Load(),
		"replay_highlight_time_ms_total": m.replayHighlightTimeMs.Load(),
		"sse_connections_current":        m.sseConnectionsCurrent.Load(),
		"sse_connections_total":          m.sseConnectionsTotal.Load(),
		"sse_disconnects_total":          m.sseDisconnectsTotal.Load(),
		"sse_data_messages_total":        m.sseDataMessagesTotal.Load(),
	}
	by := map[string]int64{}
	m.mu.Lock()
	for k, v := range m.byStatus {
		by[itoa(k)] = v.Load()
	}
	m.mu.Unlock()
	out["responses_by_status"] = by
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
