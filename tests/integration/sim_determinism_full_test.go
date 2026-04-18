package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
	"lobster-world-core/internal/gateway"
	"lobster-world-core/internal/sim"
)

func TestDeterminism_SameSeedDeepEqualityAcrossRuns(t *testing.T) {
	t.Parallel()

	cfg := &sim.ShockConfig{
		// Small epoch to keep test fast.
		EpochTicks:    6,
		WarningOffset: 1,
		DurationTicks: 2,
		CooldownTicks: 6,
		Candidates: []sim.ShockCandidate{
			{
				Key:              "riftwinter",
				Weight:           1,
				WarningNarrative:  "天象异常：裂冬指数上升",
				StartedNarrative:  "冲击开始：裂冬纪元降临",
				EndedNarrative:    "冲击结束：裂冬余波仍在",
				ActorsPool:        []string{"nation_a", "nation_b", "nation_c"},
			},
		},
	}

	app1 := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond, Shock: cfg, Seed: 777})
	app2 := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 20 * time.Millisecond, Shock: cfg, Seed: 777})

	s1 := httptest.NewServer(app1.Handler)
	t.Cleanup(s1.Close)
	s2 := httptest.NewServer(app2.Handler)
	t.Cleanup(s2.Close)

	worldID := "w_det_deep"

	// Same semantic input sequence, different wall-clock pacing.
	submitGoal(t, s1.URL, worldID, "启动世界")
	submitGoal(t, s1.URL, worldID, "研究农业")
	submitGoal(t, s1.URL, worldID, "扩张边境")

	submitGoal(t, s2.URL, worldID, "启动世界")
	time.Sleep(7 * time.Millisecond)
	submitGoal(t, s2.URL, worldID, "研究农业")
	time.Sleep(3 * time.Millisecond)
	submitGoal(t, s2.URL, worldID, "扩张边境")

	// Wait until we have enough events on both sides (poll to avoid flakiness).
	events1 := waitEvents(t, app1.EventStore, worldID, 14, 1800*time.Millisecond)
	events2 := waitEvents(t, app2.EventStore, worldID, 14, 1800*time.Millisecond)

	// Compare first N events deeply (including ts/delta/trace/meta).
	n := 14
	if len(events1) < n {
		n = len(events1)
	}
	if len(events2) < n {
		n = len(events2)
	}
	if n < 10 {
		t.Fatalf("expected enough events to compare, got %d/%d", len(events1), len(events2))
	}

	for i := 0; i < n; i++ {
		a := normalize(events1[i])
		b := normalize(events2[i])
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("event mismatch at %d:\nA=%#v\nB=%#v", i, a, b)
		}
	}

	// Business-level assertions: shock lifecycle must be complete and consistent.
	// We expect (at least once): shock_warning -> shock_started -> shock_ended -> betrayal.
	types1 := typeSet(events1)
	for _, typ := range []string{"shock_warning", "shock_started", "shock_ended", "betrayal"} {
		if !types1[typ] {
			t.Fatalf("expected events include %s, got %#v", typ, types1)
		}
	}

	// Ensure betrayal has delta+trace and meta.shock_key.
	for _, e := range events1 {
		if e.Type != "betrayal" {
			continue
		}
		if len(e.Actors) < 2 {
			t.Fatalf("betrayal must have 2 actors, got %#v", e.Actors)
		}
		if e.Meta == nil || e.Meta["shock_key"] == nil {
			t.Fatalf("betrayal must have meta.shock_key, got %#v", e.Meta)
		}
		if e.Delta == nil || e.Delta["trust"] == nil || e.Delta["order"] == nil || e.Delta["conflict"] == nil {
			t.Fatalf("betrayal must have delta trust/order/conflict, got %#v", e.Delta)
		}
		if len(e.Trace) < 2 {
			t.Fatalf("betrayal must have >=2 trace notes, got %#v", e.Trace)
		}
		break
	}

	// End-to-end state assertion: betrayal delta must be reflected in /world/status.
	// Baseline in sim is trust=50 order=50 conflict=0.
	st1 := getWorldStatus(t, s1.URL, worldID)
	st2 := getWorldStatus(t, s2.URL, worldID)
	if st1["trust"] >= 50 || st1["order"] >= 50 || st1["conflict"] <= 0 {
		t.Fatalf("expected world status reflects betrayal delta, got %#v", st1)
	}
	// Deterministic state under same seed + same inputs.
	if !reflect.DeepEqual(st1, st2) {
		t.Fatalf("expected deterministic world status, got %#v vs %#v", st1, st2)
	}
}

func getWorldStatus(t *testing.T, baseURL, worldID string) map[string]float64 {
	t.Helper()
	resp, err := http.Get(baseURL + "/api/v0/world/status?world_id=" + worldID)
	if err != nil {
		t.Fatalf("GET /world/status: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	state, ok := out["state"].(map[string]any)
	if !ok {
		t.Fatalf("expected state object, got %#v", out["state"])
	}
	return map[string]float64{
		"trust":    state["trust"].(float64),
		"order":    state["order"].(float64),
		"conflict": state["conflict"].(float64),
	}
}

func submitGoal(t *testing.T, baseURL, worldID, goal string) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": goal})
	r, err := http.Post(baseURL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", r.StatusCode)
	}
}

func waitEvents(t *testing.T, es store.EventStore, worldID string, minCount int, timeout time.Duration) []spec.Event {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var events []spec.Event
	for time.Now().Before(deadline) {
		out, err := es.Query(store.Query{WorldID: worldID, SinceTs: 0, Limit: 2000})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		events = out
		if len(events) >= minCount {
			return events
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for events (got %d, want >=%d)", len(events), minCount)
	return nil
}

// normalize ensures nil vs empty are comparable and strips fields that are allowed to differ.
// For P3-M3 Step2 we expect full equality including Ts, so this is minimal.
func normalize(e spec.Event) spec.Event {
	if e.Delta == nil {
		e.Delta = map[string]any{}
	}
	if e.Meta == nil {
		e.Meta = map[string]any{}
	}
	if e.Actors == nil {
		e.Actors = []string{}
	}
	if e.Trace == nil {
		e.Trace = []spec.TraceLink{}
	}
	return e
}

func typeSet(events []spec.Event) map[string]bool {
	m := map[string]bool{}
	for _, e := range events {
		m[e.Type] = true
	}
	return m
}
