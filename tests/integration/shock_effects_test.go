package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/gateway"
	"lobster-world-core/internal/sim"
)

func TestShockStarted_AppliesDeltaToWorldStatus(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		Shock: &sim.ShockConfig{
			EpochTicks:    12,
			WarningOffset: 2,
			DurationTicks: 3,
			CooldownTicks: 12,
			Candidates: []sim.ShockCandidate{
				{
					Key:              "riftwinter",
					Weight:           1,
					WarningNarrative:  "天象异常：裂冬指数上升",
					StartedNarrative:  "冲击开始：裂冬纪元降临",
					EndedNarrative:    "冲击结束：裂冬余波仍在",
					StartedDelta: map[string]int64{
						"food":     -10,
						"order":    -5,
						"conflict": +8,
					},
				},
			},
		},
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	worldID := "w_shock_delta"

	// Connect SSE first, wait for shock_started then query world/status.
	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, s.URL+"/api/v0/events/stream?world_id="+worldID, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("connect stream: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	br := bufio.NewReader(resp.Body)

	// Ensure world exists by submitting a dummy intent (so it starts ticking).
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
	r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	r.Body.Close()

	// Wait for shock_started event.
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		e := readNextDataEventShock(t, br, 1500*time.Millisecond)
		if e.Type == "shock_started" {
			break
		}
	}

	// Now query status and assert delta applied.
	stResp, err := http.Get(s.URL + "/api/v0/world/status?world_id=" + worldID)
	if err != nil {
		t.Fatalf("GET /world/status: %v", err)
	}
	defer stResp.Body.Close()
	if stResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", stResp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(stResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	state := out["state"].(map[string]any)
	food := state["food"].(float64)
	conflict := state["conflict"].(float64)
	if food >= 100 {
		t.Fatalf("expected food < 100 after shock delta, got %v", food)
	}
	if conflict <= 0 {
		t.Fatalf("expected conflict > 0 after shock delta, got %v", conflict)
	}
}

// Keep this helper local to avoid cross-test file coupling.
func readNextDataEventShock(t *testing.T, br *bufio.Reader, timeout time.Duration) spec.Event {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			var e spec.Event
			if err := json.Unmarshal([]byte(payload), &e); err != nil {
				t.Fatalf("unmarshal event: %v payload=%q", err, payload)
			}
			return e
		}
	}
	t.Fatalf("timed out waiting for data line")
	return spec.Event{}
}
