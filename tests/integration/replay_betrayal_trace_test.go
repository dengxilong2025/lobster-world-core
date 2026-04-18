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

	"lobster-world-core/internal/gateway"
	"lobster-world-core/internal/sim"
)

func TestReplayHighlight_UsesBetrayalTraceNotesFromShock(t *testing.T) {
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

	worldID := "w_replay_trace"

	// Connect SSE first to capture the betrayal event_id.
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

	var betrayalID string
	var note1, note2 string
	var a1, a2 string
	var betrayalNarrative string
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		e := readNextDataEventLocal(t, br, 1500*time.Millisecond)
		if e.Type == "betrayal" {
			if len(e.Trace) < 2 {
				t.Fatalf("expected betrayal has >=2 trace notes, got %#v", e.Trace)
			}
			if len(e.Actors) >= 2 {
				a1, a2 = e.Actors[0], e.Actors[1]
			}
			betrayalID = e.EventID
			note1 = e.Trace[0].Note
			note2 = e.Trace[1].Note
			betrayalNarrative = e.Narrative
			break
		}
	}
	if betrayalID == "" {
		t.Fatalf("timed out waiting for betrayal event")
	}

	// Call replay/highlight and ensure it uses the first two trace notes.
	rp, err := http.Get(s.URL + "/api/v0/replay/highlight?world_id=" + worldID + "&event_id=" + betrayalID)
	if err != nil {
		t.Fatalf("GET replay/highlight: %v", err)
	}
	defer rp.Body.Close()
	if rp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", rp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(rp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	beats, ok := out["beats"].([]any)
	if !ok || len(beats) < 4 {
		t.Fatalf("expected >=4 beats, got %#v", out["beats"])
	}
	b1, _ := beats[1].(map[string]any)
	b2, _ := beats[2].(map[string]any)
	if b1["caption"] != "因为："+note1 {
		t.Fatalf("expected beat[1] uses trace[0], got %#v", b1)
	}
	if b2["caption"] != "进展："+note2 {
		t.Fatalf("expected beat[2] uses trace[1], got %#v", b2)
	}

	// Stage2: ensure the replay contains an "aftermath" line referencing the relationship flip.
	foundAftermath := false
	for _, it := range beats {
		m, _ := it.(map[string]any)
		cap, _ := m["caption"].(string)
		if strings.HasPrefix(cap, "余波：") {
			foundAftermath = true
			if a1 != "" && (!strings.Contains(cap, a1) || !strings.Contains(cap, a2)) {
				t.Fatalf("expected aftermath mentions actors %q/%q, got %q", a1, a2, cap)
			}
			if betrayalNarrative != "" && !strings.Contains(cap, betrayalNarrative) {
				t.Fatalf("expected aftermath mentions reason note %q, got %q", betrayalNarrative, cap)
			}
			break
		}
	}
	if !foundAftermath {
		t.Fatalf("expected an aftermath beat starting with 余波：, got %#v", beats)
	}
}
