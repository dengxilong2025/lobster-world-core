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

func TestShockStarted_EmitsBetrayalBetweenActorsPool(t *testing.T) {
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
					ActorsPool:        []string{"nation_a", "nation_b", "nation_c"},
				},
			},
		},
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	worldID := "w_shock_betrayal"

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

	// Wait for betrayal event.
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		e := readNextDataEventBetrayal(t, br, 1500*time.Millisecond)
		if e.Type == "betrayal" {
			if len(e.Actors) < 2 {
				t.Fatalf("expected betrayal has 2 actors, got %#v", e.Actors)
			}
			// actors must come from pool
			if !inSet(e.Actors[0], map[string]bool{"nation_a": true, "nation_b": true, "nation_c": true}) ||
				!inSet(e.Actors[1], map[string]bool{"nation_a": true, "nation_b": true, "nation_c": true}) {
				t.Fatalf("actors not from pool: %#v", e.Actors)
			}
			if e.Actors[0] == e.Actors[1] {
				t.Fatalf("actors should be different: %#v", e.Actors)
			}
			// tick should be set (>= epochStart)
			if e.Tick <= 0 {
				t.Fatalf("expected tick > 0, got %d", e.Tick)
			}
			// meta should include shock_key
			if e.Meta == nil || e.Meta["shock_key"] == nil {
				t.Fatalf("expected meta.shock_key, got %#v", e.Meta)
			}
			// trace should exist for replay narration (MVP).
			if len(e.Trace) < 2 || e.Trace[0].CauseEventID == "" || strings.TrimSpace(e.Trace[0].Note) == "" || strings.TrimSpace(e.Trace[1].Note) == "" {
				t.Fatalf("expected trace note, got %#v", e.Trace)
			}

			// spectator/entity should surface betrayal as next_risk for involved entity.
			checkEntityNextRiskContains(t, s.URL, worldID, e.Actors[0], "背叛已发生：")
			checkEntityNextRiskContains(t, s.URL, worldID, e.Actors[0], "冲击期：")
			checkEntityWhyStrongContains(t, s.URL, worldID, e.Actors[0], "信誉受损：")
			checkEntityWhyStrongContains(t, s.URL, worldID, e.Actors[1], "遭遇背叛：")
			checkEntityWhyStrongContains(t, s.URL, worldID, e.Actors[0], "冲击期：")
			checkEntityWhyStrongContains(t, s.URL, worldID, e.Actors[0], "冲击预兆：")
			checkWorldStatusBetrayalDelta(t, s.URL, worldID)
			return
		}
	}
	t.Fatalf("timed out waiting for betrayal event")
}

func inSet(v string, s map[string]bool) bool { return s[v] }

func checkEntityNextRiskContains(t *testing.T, baseURL, worldID, entityID, prefix string) {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/v0/spectator/entity?world_id=" + worldID + "&entity_id=" + entityID)
	if err != nil {
		t.Fatalf("GET /spectator/entity: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	risk, ok := out["next_risk"].([]any)
	if !ok || len(risk) == 0 {
		t.Fatalf("expected next_risk, got %#v", out["next_risk"])
	}
	found := false
	for _, it := range risk {
		s, _ := it.(string)
		if strings.HasPrefix(s, prefix) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected next_risk prefix %q, got %#v", prefix, risk)
	}
}

func checkEntityWhyStrongContains(t *testing.T, baseURL, worldID, entityID, prefix string) {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/v0/spectator/entity?world_id=" + worldID + "&entity_id=" + entityID)
	if err != nil {
		t.Fatalf("GET /spectator/entity: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	why, ok := out["why_strong"].([]any)
	if !ok || len(why) == 0 {
		t.Fatalf("expected why_strong, got %#v", out["why_strong"])
	}
	found := false
	for _, it := range why {
		s, _ := it.(string)
		if strings.HasPrefix(s, prefix) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected why_strong prefix %q, got %#v", prefix, why)
	}
}

func checkWorldStatusBetrayalDelta(t *testing.T, baseURL, worldID string) {
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
	trust, _ := state["trust"].(float64)
	order, _ := state["order"].(float64)
	conflict, _ := state["conflict"].(float64)

	// Baseline in sim is trust=50, order=50, conflict=0; betrayal should worsen them.
	if trust >= 50 {
		t.Fatalf("expected trust < 50 after betrayal delta, got %v", trust)
	}
	if order >= 50 {
		t.Fatalf("expected order < 50 after betrayal delta, got %v", order)
	}
	if conflict <= 0 {
		t.Fatalf("expected conflict > 0 after betrayal delta, got %v", conflict)
	}
}

// Local helper duplicated on purpose to keep each test file self-contained.
func readNextDataEventBetrayal(t *testing.T, br *bufio.Reader, timeout time.Duration) spec.Event {
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
