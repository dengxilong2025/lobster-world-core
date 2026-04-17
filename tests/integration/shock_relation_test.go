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

func TestBetrayal_OverridesAllianceRelationToEnemy(t *testing.T) {
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
					// Fixed pool so betrayal is deterministic between these two.
					ActorsPool: []string{"nation_a", "nation_b"},
				},
			},
		},
	})

	worldID := "w_rel"

	// Pre-existing alliance.
	_ = app.EventStore.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_alliance_1",
		Ts:            1,
		WorldID:       worldID,
		Scope:         "world",
		Type:          "alliance_formed",
		Actors:        []string{"nation_a", "nation_b"},
		Narrative:     "血盟成立：A与B结盟",
	})

	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	// Connect SSE and wait for betrayal.
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
	var betrayalID string
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		e := readNextDataEventRel(t, br, 1500*time.Millisecond)
		if e.Type == "betrayal" {
			betrayalID = e.EventID
			break
		}
	}
	if betrayalID == "" {
		t.Fatalf("timed out waiting for betrayal event")
	}

	// Verify spectator relation is enemy (betrayal overrides alliance).
	resp2, err := http.Get(s.URL + "/api/v0/spectator/entity?world_id=" + worldID + "&entity_id=nation_a")
	if err != nil {
		t.Fatalf("GET spectator/entity: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	var out struct {
		OK        bool `json:"ok"`
		Relations []struct {
			To   string `json:"to"`
			Type string `json:"type"`
		} `json:"relations"`
		RelationReasons []struct {
			To      string `json:"to"`
			Type    string `json:"type"`
			EventID string `json:"event_id"`
			Note    string `json:"note"`
		} `json:"relation_reasons"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK {
		t.Fatalf("expected ok=true")
	}
	found := false
	for _, r := range out.Relations {
		if r.To == "nation_b" {
			found = true
			if r.Type != "enemy" {
				t.Fatalf("expected relation enemy, got %s", r.Type)
			}
		}
	}
	if !found {
		t.Fatalf("expected relation to nation_b, got %#v", out.Relations)
	}

	// Also verify we expose the reason (event_id) for the flip.
	foundReason := false
	for _, rr := range out.RelationReasons {
		if rr.To == "nation_b" && rr.Type == "enemy" {
			foundReason = true
			if rr.EventID != betrayalID {
				t.Fatalf("expected reason event_id=%s, got %s", betrayalID, rr.EventID)
			}
			if rr.Note == "" {
				t.Fatalf("expected non-empty reason note")
			}
		}
	}
	if !foundReason {
		t.Fatalf("expected relation_reasons entry for nation_b enemy, got %#v", out.RelationReasons)
	}
}

func readNextDataEventRel(t *testing.T, br *bufio.Reader, timeout time.Duration) spec.Event {
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
