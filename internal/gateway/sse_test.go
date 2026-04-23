package gateway

import (
	"bufio"
	"errors"
	"testing"
)

type errWriter struct {
	fail bool
}

func (w *errWriter) Write(p []byte) (int, error) {
	if w.fail {
		return 0, errors.New("write failed")
	}
	return len(p), nil
}

type noopFlusher struct{}

func (noopFlusher) Flush() {}

func TestWriteSSEMessage_ReturnsErrorWhenUnderlyingWriterFails(t *testing.T) {
	t.Parallel()

	ew := &errWriter{fail: true}
	bw := bufio.NewWriter(ew)
	_, err := writeSSEMessage(bw, noopFlusher{}, []byte(`{"ok":true}`))
	if err == nil {
		t.Fatalf("expected error")
	}
}
