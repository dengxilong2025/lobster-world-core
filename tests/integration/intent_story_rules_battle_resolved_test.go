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

type exportEventBR struct {
	spec.Event
	ExportSchemaVersion int `json:"export_schema_version"`
}

func readExportBR(t *testing.T, baseURL, worldID string) []exportEventBR {
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
	out := []exportEventBR{}
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev exportEventBR
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

func findByTypeBR(evs []exportEventBR, typ string) *exportEventBR {
	for i := range evs {
		if evs[i].Type == typ {
			return &evs[i]
		}
	}
	return nil
}

func findActionCompletedIDsBR(evs []exportEventBR) map[string]struct{} {
	ids := map[string]struct{}{}
	for i := range evs {
		if evs[i].Type == "action_completed" {
			ids[evs[i].EventID] = struct{}{}
		}
	}
	return ids
}

func hasTraceCauseInSetBR(e exportEventBR, ids map[string]struct{}) bool {
	for _, tl := range e.Trace {
		if _, ok := ids[tl.CauseEventID]; ok {
			return true
		}
	}
	return false
}

func numBR(m map[string]any, k string) (int64, bool) {
	v, ok := m[k]
	if !ok || v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	default:
		return 0, false
	}
}

func TestStoryRules_BattleResolved(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		Seed:         2468,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_story_battle_resolved"

	b, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "进攻：发动会战"})
	r, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	_ = r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("intent status=%d", r.StatusCode)
	}

	time.Sleep(300 * time.Millisecond)

	evs := readExportBR(t, s.URL, worldID)
	acIDs := findActionCompletedIDsBR(evs)
	if len(acIDs) == 0 {
		t.Fatalf("missing action_completed in export")
	}

	br := findByTypeBR(evs, "battle_resolved")
	if br == nil {
		t.Fatalf("missing battle_resolved in export")
	}
	if len(br.Actors) != 2 || br.Actors[0] == br.Actors[1] {
		t.Fatalf("battle_resolved actors invalid: %v", br.Actors)
	}
	if !hasTraceCauseInSetBR(*br, acIDs) {
		t.Fatalf("battle_resolved should trace some action_completed")
	}
	if v, ok := numBR(br.Delta, "conflict"); !ok || v <= 0 {
		t.Fatalf("battle_resolved expected conflict>0 delta=%v", br.Delta)
	}

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
	if !strings.Contains(hs, "battle_resolved") {
		t.Fatalf("expected home hints mention battle_resolved, got=%s", hs)
	}
}

