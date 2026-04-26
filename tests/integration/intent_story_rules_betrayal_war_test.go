package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/gateway"
)

type exportEventBW struct {
	spec.Event
	ExportSchemaVersion int `json:"export_schema_version"`
}

func readExportBW(t *testing.T, baseURL, worldID string) []exportEventBW {
	t.Helper()
	resp, err := http.Get(baseURL + "/api/v0/replay/export?world_id=" + worldID + "&limit=5000")
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("export status=%d body=%q", resp.StatusCode, string(b))
	}

	sc := bufio.NewScanner(resp.Body)
	out := []exportEventBW{}
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev exportEventBW
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatalf("unmarshal: %v line=%q", err, string(line))
		}
		if err := ev.Event.Validate(); err != nil {
			t.Fatalf("invalid event: %v ev=%#v", err, ev.Event)
		}
		if ev.ExportSchemaVersion <= 0 {
			t.Fatalf("expected export_schema_version>0")
		}
		out = append(out, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return out
}

func findByTypeBW(evs []exportEventBW, typ string) *exportEventBW {
	for i := range evs {
		if evs[i].Type == typ {
			return &evs[i]
		}
	}
	return nil
}

func findActionCompletedBW(evs []exportEventBW) *exportEventBW {
	for i := range evs {
		if evs[i].Type == "action_completed" {
			return &evs[i]
		}
	}
	return nil
}

func actionCompletedIDsBW(evs []exportEventBW) map[string]struct{} {
	out := map[string]struct{}{}
	for i := range evs {
		if evs[i].Type == "action_completed" && evs[i].EventID != "" {
			out[evs[i].EventID] = struct{}{}
		}
	}
	return out
}

func hasTraceCauseBW(e exportEventBW, causeEventID string) bool {
	for _, tl := range e.Trace {
		if tl.CauseEventID == causeEventID {
			return true
		}
	}
	return false
}

func hasTraceCauseInSetBW(e exportEventBW, causeEventIDs map[string]struct{}) bool {
	for _, tl := range e.Trace {
		if _, ok := causeEventIDs[tl.CauseEventID]; ok {
			return true
		}
	}
	return false
}

func numBW(m map[string]any, k string) (int64, bool) {
	v, ok := m[k]
	if !ok || v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64: // JSON numbers sometimes come as float64 in loose decoding
		return int64(x), true
	default:
		return 0, false
	}
}

func TestStoryRules_BetrayalAndWarStarted(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		Seed:         456,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_story_bw"

	postIntent := func(goal string) {
		b, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": goal})
		r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("POST /intents: %v", err)
		}
		_ = r.Body.Close()
		if r.StatusCode != http.StatusOK {
			t.Fatalf("intent status=%d goal=%q", r.StatusCode, goal)
		}
	}

	// Trigger betrayal and war.
	postIntent("背叛：翻脸")
	postIntent("宣战：开战")
	time.Sleep(350 * time.Millisecond)

	evs := readExportBW(t, s.URL, worldID)
	ac := findActionCompletedBW(evs)
	if ac == nil {
		t.Fatalf("missing action_completed in export")
	}
	acIDs := actionCompletedIDsBW(evs)

	be := findByTypeBW(evs, "betrayal")
	if be == nil {
		t.Fatalf("missing betrayal in export")
	}
	if len(be.Actors) != 2 || be.Actors[0] == be.Actors[1] {
		t.Fatalf("betrayal actors invalid: %v", be.Actors)
	}
	if !hasTraceCauseInSetBW(*be, acIDs) {
		t.Fatalf("betrayal should trace an action_completed, trace=%#v", be.Trace)
	}
	if v, ok := numBW(be.Delta, "trust"); !ok || v >= 0 {
		t.Fatalf("betrayal expected trust<0 delta=%v", be.Delta)
	}
	if v, ok := numBW(be.Delta, "conflict"); !ok || v <= 0 {
		t.Fatalf("betrayal expected conflict>0 delta=%v", be.Delta)
	}

	we := findByTypeBW(evs, "war_started")
	if we == nil {
		t.Fatalf("missing war_started in export")
	}
	if len(we.Actors) != 2 || we.Actors[0] == we.Actors[1] {
		t.Fatalf("war_started actors invalid: %v", we.Actors)
	}
	if !hasTraceCauseInSetBW(*we, acIDs) {
		t.Fatalf("war_started should trace an action_completed, trace=%#v", we.Trace)
	}
	if v, ok := numBW(we.Delta, "conflict"); !ok || v <= 0 {
		t.Fatalf("war_started expected conflict>0 delta=%v", we.Delta)
	}

	// Home should be actionable and mention at least one expected event type under conflict/betrayal context.
	hr, err := http.Get(s.URL + "/api/v0/spectator/home?world_id=" + worldID)
	if err != nil {
		t.Fatalf("GET home: %v", err)
	}
	defer hr.Body.Close()
	hb, _ := io.ReadAll(hr.Body)
	hs := string(hb)
	if !strings.Contains(hs, "建议：") {
		t.Fatalf("expected hints in home, got=%s", hs)
	}
	if !(strings.Contains(hs, "betrayal") || strings.Contains(hs, "war_started")) {
		t.Fatalf("expected home hints mention betrayal/war_started, got=%s", hs)
	}
}
