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

func TestShockScheduler_EmitsWarningStartedEnded(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		Shock: &sim.ShockConfig{
			EpochTicks:    12,
			WarningOffset: 2,
			DurationTicks: 3,
			CooldownTicks: 12,
			Candidates: []sim.ShockCandidate{
				{Key: "riftwinter", Weight: 1, WarningNarrative: "天象异常：裂冬指数上升", StartedNarrative: "冲击开始：裂冬纪元降临", EndedNarrative: "冲击结束：裂冬余波仍在"},
			},
		},
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	worldID := "w_shock"

	// Connect SSE first.
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

	// We expect at least: shock_warning -> shock_started -> shock_ended (order by tick).
	var got []spec.Event
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) && len(got) < 3 {
		e := readNextDataEventLocal(t, br, 1500*time.Millisecond)
		if e.Type == "shock_warning" || e.Type == "shock_started" || e.Type == "shock_ended" {
			got = append(got, e)
		}
	}
	if len(got) < 3 {
		t.Fatalf("expected 3 shock events, got %d", len(got))
	}
	if got[0].Type != "shock_warning" {
		t.Fatalf("expected first shock_warning, got %s", got[0].Type)
	}
	if got[1].Type != "shock_started" {
		t.Fatalf("expected second shock_started, got %s", got[1].Type)
	}
	if got[2].Type != "shock_ended" {
		t.Fatalf("expected third shock_ended, got %s", got[2].Type)
	}
	if !(got[0].Tick < got[1].Tick && got[1].Tick <= got[2].Tick) {
		t.Fatalf("unexpected tick order: %d, %d, %d", got[0].Tick, got[1].Tick, got[2].Tick)
	}
	if !strings.Contains(got[0].Narrative, "裂冬") {
		t.Fatalf("expected narrative contains keyword, got %q", got[0].Narrative)
	}
}
