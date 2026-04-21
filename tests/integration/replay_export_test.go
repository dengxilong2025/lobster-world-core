package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/gateway"
	"lobster-world-core/internal/sim"
)

func TestReplayExport_ReturnsStableNDJSONSorted(t *testing.T) {
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

	worldID := "w_export"

	// Kick the world so we have a few events.
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
	r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	r.Body.Close()

	// Wait a bit so shock lifecycle can happen.
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(s.URL + "/api/v0/replay/export?world_id=" + worldID)
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body1, _ := io.ReadAll(resp.Body)

	// Parse NDJSON.
	sc := bufio.NewScanner(bytes.NewReader(body1))
	var events []spec.Event
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		// Ensure export format versioning exists (backward compatible: only adds fields).
		var meta map[string]any
		if err := json.Unmarshal(line, &meta); err != nil {
			t.Fatalf("invalid json line: %v line=%q", err, string(line))
		}
		if v, ok := meta["export_schema_version"]; !ok || v == nil {
			t.Fatalf("expected export_schema_version field")
		}
		var e spec.Event
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("invalid json line: %v line=%q", err, string(line))
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("invalid event: %v e=%#v", err, e)
		}
		events = append(events, e)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(events) < 6 {
		t.Fatalf("expected some events, got %d", len(events))
	}

	// Assert sorted by (ts asc, event_id asc).
	for i := 1; i < len(events); i++ {
		a := events[i-1]
		b := events[i]
		if a.Ts > b.Ts || (a.Ts == b.Ts && a.EventID > b.EventID) {
			t.Fatalf("events not sorted at %d: prev=%#v next=%#v", i, a, b)
		}
	}

	// Deterministic: same request twice yields identical output.
	resp2, err := http.Get(s.URL + "/api/v0/replay/export?world_id=" + worldID)
	if err != nil {
		t.Fatalf("GET export2: %v", err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	if string(body1) != string(body2) {
		t.Fatalf("expected deterministic export output")
	}
}

func TestReplayExport_EmptyWorld_Returns200AndEmptyBody(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 10 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	worldID := "w_empty_export"
	resp, err := http.Get(s.URL + "/api/v0/replay/export?world_id=" + worldID)
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if len(bytes.TrimSpace(b)) != 0 {
		t.Fatalf("expected empty body, got %q", string(b))
	}
}

func TestReplayExport_Limit_IsHonored(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 10 * time.Millisecond,
		Seed:         123,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	worldID := "w_export_limit"
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "启动世界"})
	for i := 0; i < 5; i++ {
		r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST /intents: %v", err)
		}
		r.Body.Close()
	}
	time.Sleep(200 * time.Millisecond)

	resp, err := http.Get(s.URL + "/api/v0/replay/export?world_id=" + worldID + "&limit=3")
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	n := 0
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) == 0 {
			continue
		}
		n++
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if n > 3 {
		t.Fatalf("expected <=3 events, got %d", n)
	}
}

func TestReplayExport_Limit_NonPositive_IsRejected(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{TickInterval: 10 * time.Millisecond})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	resp, err := http.Get(s.URL + "/api/v0/replay/export?world_id=w1&limit=0")
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
