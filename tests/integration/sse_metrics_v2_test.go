package integration

import (
	"bufio"
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func TestSSEMetricsV2_ByWorldBytesAndDuration(t *testing.T) {
	// No t.Parallel(): involves timing and streaming.
	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 5 * time.Millisecond, Seed: 123})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_sse_v2"

	// connect SSE
	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/v0/events/stream?world_id="+worldID, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}

	br := bufio.NewReader(resp.Body)
	// read first line ":ok\n" so stream is established
	_, _ = br.ReadString('\n')

	before := getMetricsMap(t, s.URL)
	bBytes := metricInt64(t, before, "sse_bytes_total")
	bDurCnt := metricInt64(t, before, "sse_conn_duration_count")

	// by-world gauge present
	by, ok := before["sse_connections_current_by_world"].(map[string]any)
	if !ok || by == nil {
		t.Fatalf("expected sse_connections_current_by_world")
	}
	if int64(by[worldID].(float64)) < 1 {
		t.Fatalf("expected by_world>=1")
	}

	// trigger at least one event
	body := []byte(`{"world_id":"` + worldID + `","goal":"启动世界"}`)
	r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post intent: %v", err)
	}
	_ = r.Body.Close()

	// read one SSE message line "data: ..."
	deadline := time.Now().Add(800 * time.Millisecond)
	found := false
	for time.Now().Before(deadline) {
		line, _ := br.ReadString('\n')
		if len(line) >= 6 && line[:6] == "data: " {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to receive at least one data: line")
	}

	mid := getMetricsMap(t, s.URL)
	if metricInt64(t, mid, "sse_bytes_total") <= bBytes {
		t.Fatalf("expected sse_bytes_total to increase")
	}

	// close and ensure duration count increments
	_ = resp.Body.Close()
	time.Sleep(50 * time.Millisecond)

	after := getMetricsMap(t, s.URL)
	if metricInt64(t, after, "sse_conn_duration_count") <= bDurCnt {
		t.Fatalf("expected conn_duration_count to increase")
	}
}

