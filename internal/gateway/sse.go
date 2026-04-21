package gateway

import (
	"bufio"
	"net/http"
)

// writeSSEMessage writes one SSE "message" event and flushes it.
// It returns any write/flush error so callers can stop streaming on disconnects.
func writeSSEMessage(bw *bufio.Writer, flusher http.Flusher, data []byte) error {
	if _, err := bw.WriteString("event: message\n"); err != nil {
		return err
	}
	if _, err := bw.WriteString("data: "); err != nil {
		return err
	}
	if _, err := bw.Write(data); err != nil {
		return err
	}
	if _, err := bw.WriteString("\n\n"); err != nil {
		return err
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

