package gateway

import (
	"bufio"
	"net/http"
)

// writeSSEMessage writes one SSE "message" event and flushes it.
// It returns any write/flush error so callers can stop streaming on disconnects.
func writeSSEMessage(bw *bufio.Writer, flusher http.Flusher, data []byte) (int64, error) {
	var n int64
	if m, err := bw.WriteString("event: message\n"); err != nil {
		return n, err
	} else {
		n += int64(m)
	}
	if m, err := bw.WriteString("data: "); err != nil {
		return n, err
	} else {
		n += int64(m)
	}
	if m, err := bw.Write(data); err != nil {
		return n, err
	} else {
		n += int64(m)
	}
	if m, err := bw.WriteString("\n\n"); err != nil {
		return n, err
	} else {
		n += int64(m)
	}
	if err := bw.Flush(); err != nil {
		return n, err
	}
	flusher.Flush()
	return n, nil
}
