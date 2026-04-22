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

func (m *Metrics) Snapshot() map[string]any {
	out := map[string]any{
		"requests_total": m.requestsTotal.Load(),
		"busy_total":     m.busyTotal.Load(),
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

// defaultMetrics is owned by the gateway process (one per handler wiring).
// We keep it package-level so writeError can increment BUSY without threading through every callsite.
var defaultMetrics *Metrics

func setDefaultMetrics(m *Metrics) {
	defaultMetrics = m
}

func getDefaultMetrics() *Metrics {
	return defaultMetrics
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

