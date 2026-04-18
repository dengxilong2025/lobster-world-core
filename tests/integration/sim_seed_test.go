package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
	"lobster-world-core/internal/sim"
)

func TestWorldSeed_ControlsBetrayalActorSelection(t *testing.T) {
	t.Parallel()

	// Same shock config, varying seed should yield more than one betrayal pair across runs.
	// (We avoid asserting a specific pair per seed to prevent accidental collisions.)
	cfg := &sim.ShockConfig{
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
	}

	seen := map[string]bool{}
	// Use enough seeds to make collisions extremely unlikely if seed is actually used.
	for _, seed := range []int64{11, 22, 33, 44, 55, 66} {
		a := startSeededApp(t, seed, cfg)
		pair := waitForBetrayalPair(t, a.baseURL, a.worldID)
		seen[pair] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected >=2 distinct betrayal pairs across seeds, got %#v", seen)
	}
}

func TestDeterminism_SameSeedSameShockKeyAndBetrayalPair(t *testing.T) {
	t.Parallel()

	// Two candidates so the chosen shock_key is also seed-controlled.
	cfg := &sim.ShockConfig{
		EpochTicks:    12,
		WarningOffset: 2,
		DurationTicks: 3,
		CooldownTicks: 12,
		Candidates: []sim.ShockCandidate{
			{Key: "riftwinter", Weight: 1, WarningNarrative: "天象异常：裂冬指数上升", StartedNarrative: "冲击开始：裂冬纪元降临", EndedNarrative: "冲击结束：裂冬余波仍在", ActorsPool: []string{"nation_a", "nation_b", "nation_c"}},
			{Key: "bloodmoon", Weight: 1, WarningNarrative: "血月将至：风声鹤唳", StartedNarrative: "冲击开始：血月照临", EndedNarrative: "冲击结束：血月退潮", ActorsPool: []string{"nation_a", "nation_b", "nation_c"}},
		},
	}

	seed := int64(4242)
	a1 := startSeededApp(t, seed, cfg)
	time.Sleep(33 * time.Millisecond) // wall-clock difference should not matter
	a2 := startSeededApp(t, seed, cfg)

	pair1, key1 := waitForBetrayalPairAndShockKey(t, a1.baseURL, a1.worldID)
	pair2, key2 := waitForBetrayalPairAndShockKey(t, a2.baseURL, a2.worldID)

	if pair1 != pair2 {
		t.Fatalf("expected same betrayal pair for same seed, got %q vs %q", pair1, pair2)
	}
	if key1 != key2 {
		t.Fatalf("expected same shock_key for same seed, got %q vs %q", key1, key2)
	}
}

func TestDeterminism_DifferentSeedsProduceDifferentOutcomes(t *testing.T) {
	t.Parallel()

	cfg := &sim.ShockConfig{
		EpochTicks:    12,
		WarningOffset: 2,
		DurationTicks: 3,
		CooldownTicks: 12,
		Candidates: []sim.ShockCandidate{
			{Key: "riftwinter", Weight: 1, WarningNarrative: "天象异常：裂冬指数上升", StartedNarrative: "冲击开始：裂冬纪元降临", EndedNarrative: "冲击结束：裂冬余波仍在", ActorsPool: []string{"nation_a", "nation_b", "nation_c"}},
			{Key: "bloodmoon", Weight: 1, WarningNarrative: "血月将至：风声鹤唳", StartedNarrative: "冲击开始：血月照临", EndedNarrative: "冲击结束：血月退潮", ActorsPool: []string{"nation_a", "nation_b", "nation_c"}},
		},
	}

	seen := map[string]bool{}
	for _, seed := range []int64{101, 202, 303, 404, 505, 606, 707, 808} {
		a := startSeededApp(t, seed, cfg)
		pair, key := waitForBetrayalPairAndShockKey(t, a.baseURL, a.worldID)
		seen[key+"|"+pair] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected >=2 distinct outcomes across seeds, got %#v", seen)
	}
}

type seededApp struct {
	baseURL string
	worldID string
}

func startSeededApp(t *testing.T, seed int64, cfg *sim.ShockConfig) seededApp {
	t.Helper()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		Shock:        cfg,
		Seed:         seed,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	worldID := "w_seed"
	// Kick the world by submitting a dummy intent.
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
	r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	r.Body.Close()

	return seededApp{baseURL: s.URL, worldID: worldID}
}

func waitForBetrayalPair(t *testing.T, baseURL, worldID string) string {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v0/events/stream?world_id="+worldID, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("connect stream: %v", err)
	}
	defer resp.Body.Close()
	br := bufio.NewReader(resp.Body)

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		e := readNextDataEventLocal(t, br, 1500*time.Millisecond)
		if e.Type == "betrayal" && len(e.Actors) >= 2 {
			return e.Actors[0] + "->" + e.Actors[1]
		}
	}
	t.Fatalf("timed out waiting for betrayal event")
	return ""
}

func waitForBetrayalPairAndShockKey(t *testing.T, baseURL, worldID string) (pair string, shockKey string) {
	t.Helper()

	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v0/events/stream?world_id="+worldID, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("connect stream: %v", err)
	}
	defer resp.Body.Close()
	br := bufio.NewReader(resp.Body)

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		e := readNextDataEventLocal(t, br, 1500*time.Millisecond)
		if e.Type == "betrayal" && len(e.Actors) >= 2 {
			pair = e.Actors[0] + "->" + e.Actors[1]
			if e.Meta != nil {
				if v, ok := e.Meta["shock_key"]; ok {
					shockKey, _ = v.(string)
				}
			}
			return pair, shockKey
		}
	}
	t.Fatalf("timed out waiting for betrayal event")
	return "", ""
}
