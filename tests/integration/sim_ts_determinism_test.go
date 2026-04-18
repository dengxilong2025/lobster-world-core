package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/gateway"
	"lobster-world-core/internal/sim"
)

func TestDeterministicTs_SameSeedSameInputsYieldSameEventStream(t *testing.T) {
	t.Parallel()

	cfg := &sim.ShockConfig{
		EpochTicks:    12,
		WarningOffset: 2,
		DurationTicks: 3,
		CooldownTicks: 12,
		Candidates: []sim.ShockCandidate{
			{Key: "riftwinter", Weight: 1, WarningNarrative: "天象异常：裂冬指数上升", StartedNarrative: "冲击开始：裂冬纪元降临", EndedNarrative: "冲击结束：裂冬余波仍在", ActorsPool: []string{"nation_a", "nation_b", "nation_c"}},
		},
	}

	app1 := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 5 * time.Millisecond, Shock: cfg, Seed: 999})
	app2 := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 5 * time.Millisecond, Shock: cfg, Seed: 999})

	s1 := httptest.NewServer(app1.Handler)
	t.Cleanup(s1.Close)
	s2 := httptest.NewServer(app2.Handler)
	t.Cleanup(s2.Close)

	worldID := "w_det"
	submitIntent(t, s1.URL, worldID)
	// Force wall-clock divergence between two runs. With non-deterministic Ts (time.Now),
	// the first few events will likely differ by >=1s. With deterministic Ts, they should still match.
	time.Sleep(1100 * time.Millisecond)
	submitIntent(t, s2.URL, worldID)

	// Allow a few ticks so shocks + betrayal can happen (poll to avoid flakiness).
	var events1, events2 []spec.Event
	deadline := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(deadline) {
		events1 = queryWorld(t, app1.EventStore, worldID)
		events2 = queryWorld(t, app2.EventStore, worldID)
		if len(events1) >= 8 && len(events2) >= 8 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Compare the first N events deterministically (including ts).
	n := 10
	if len(events1) < n {
		n = len(events1)
	}
	if len(events2) < n {
		n = len(events2)
	}
	if n < 6 {
		t.Fatalf("expected enough events to compare, got %d/%d", len(events1), len(events2))
	}
	for i := 0; i < n; i++ {
		a := events1[i]
		b := events2[i]
		if a.EventID != b.EventID || a.Ts != b.Ts || a.Type != b.Type || a.Narrative != b.Narrative {
			t.Fatalf("mismatch at %d:\nA=%#v\nB=%#v", i, a, b)
		}
	}
}

func submitIntent(t *testing.T, baseURL, worldID string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
	r, err := http.Post(baseURL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", r.StatusCode)
	}
}

func queryWorld(t *testing.T, es store.EventStore, worldID string) []spec.Event {
	t.Helper()
	events, err := es.Query(store.Query{WorldID: worldID, SinceTs: 0, Limit: 1000})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	return events
}
