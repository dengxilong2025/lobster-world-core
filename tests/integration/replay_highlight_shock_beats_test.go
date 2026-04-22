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

func TestReplayHighlight_ShockLifecycleBeatsAreStable(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 10 * time.Millisecond,
		Seed:         123,
		Shock: &sim.ShockConfig{
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
		},
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_highlight_shock"
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
	r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	r.Body.Close()

	// Wait: warning/start/end should happen in small-tick schedule.
	time.Sleep(250 * time.Millisecond)

	// Export and pick a shock_started event id.
	resp, err := http.Get(s.URL + "/api/v0/replay/export?world_id=" + worldID + "&limit=5000")
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	defer resp.Body.Close()

	shockStartedID := ""
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var meta map[string]any
		_ = json.Unmarshal(line, &meta)
		if meta["type"] == "shock_started" {
			if id, ok := meta["event_id"].(string); ok && id != "" {
				shockStartedID = id
				break
			}
		}
	}
	if shockStartedID == "" {
		t.Fatalf("expected to find shock_started in export")
	}

	// Call highlight.
	hr, err := http.Get(s.URL + "/api/v0/replay/highlight?world_id=" + worldID + "&event_id=" + shockStartedID)
	if err != nil {
		t.Fatalf("GET highlight: %v", err)
	}
	defer hr.Body.Close()
	if hr.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", hr.StatusCode)
	}
	var out struct {
		Beats []struct {
			T       int    `json:"t"`
			Caption string `json:"caption"`
		} `json:"beats"`
	}
	if err := json.NewDecoder(hr.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	hasOpener := false
	hasWarn := false
	hasStart := false
	hasEnd := false
	hasEnding := false
	for _, b := range out.Beats {
		if b.T == 0 && strings.TrimSpace(b.Caption) != "" {
			hasOpener = true
		}
		if strings.Contains(b.Caption, "冲击预警") || strings.Contains(b.Caption, "天象异常") {
			hasWarn = true
		}
		if strings.Contains(b.Caption, "冲击开始") || strings.Contains(b.Caption, "裂冬纪元") {
			hasStart = true
		}
		if strings.Contains(b.Caption, "冲击结束") || strings.Contains(b.Caption, "裂冬余波") {
			hasEnd = true
		}
		if strings.Contains(b.Caption, "下一步：关注冲击/背叛/迁徙窗口") {
			hasEnding = true
		}
	}
	if !hasOpener || !hasStart || !hasEnd || !hasEnding {
		t.Fatalf("expected opener/start/end/ending beats, got=%#v", out.Beats)
	}
	// Prefer to have warning too (soft assertion; keep as warning in test output).
	if !hasWarn {
		t.Logf("warning beat missing (non-fatal): beats=%#v", out.Beats)
	}
}

