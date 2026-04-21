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
)

type exportEvent struct {
	spec.Event
	ExportSchemaVersion int `json:"export_schema_version"`
}

func readExport(t *testing.T, baseURL, worldID string) []exportEvent {
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
	out := []exportEvent{}
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev exportEvent
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

func findByType(evs []exportEvent, typ string) *exportEvent {
	for i := range evs {
		if evs[i].Type == typ {
			return &evs[i]
		}
	}
	return nil
}

func findActionCompleted(evs []exportEvent) *exportEvent {
	for i := range evs {
		if evs[i].Type == "action_completed" {
			return &evs[i]
		}
	}
	return nil
}

func hasTraceCause(e exportEvent, causeEventID string) bool {
	for _, tl := range e.Trace {
		if tl.CauseEventID == causeEventID {
			return true
		}
	}
	return false
}

func num(m map[string]any, k string) (int64, bool) {
	v, ok := m[k]
	if !ok || v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return int64(x), true
	case int64:
		return x, true
	case int:
		return int64(x), true
	default:
		return 0, false
	}
}

func TestIntentStoryRules_Diplomacy_EmitsAllianceFormed(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 10 * time.Millisecond,
		Seed:         123,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_story_diplomacy"
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "发起结盟"})
	resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	resp.Body.Close()
	time.Sleep(120 * time.Millisecond)

	evs := readExport(t, s.URL, worldID)
	ac := findActionCompleted(evs)
	if ac == nil {
		t.Fatalf("expected action_completed in export")
	}
	al := findByType(evs, "alliance_formed")
	if al == nil {
		t.Fatalf("expected alliance_formed in export")
	}
	if al.Ts <= ac.Ts {
		t.Fatalf("expected alliance_formed after action_completed (ts), got ac.ts=%d al.ts=%d", ac.Ts, al.Ts)
	}
	if len(al.Actors) != 2 || al.Actors[0] == al.Actors[1] {
		t.Fatalf("expected 2 distinct actors, got %#v", al.Actors)
	}
	if _, ok := num(al.Delta, "trust"); !ok {
		t.Fatalf("expected delta.trust in alliance_formed, got %#v", al.Delta)
	}
	if !hasTraceCause(*al, ac.EventID) {
		t.Fatalf("expected alliance_formed trace links to action_completed %q", ac.EventID)
	}
}

func TestIntentStoryRules_Trade_EmitsTradeAgreement(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 10 * time.Millisecond,
		Seed:         123,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_story_trade"
	body, _ := json.Marshal(map[string]any{"world_id": worldID, "goal": "组织集市交换物资"})
	resp, err := http.Post(s.URL+"/api/v0/intents", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /intents: %v", err)
	}
	resp.Body.Close()
	time.Sleep(120 * time.Millisecond)

	evs := readExport(t, s.URL, worldID)
	ac := findActionCompleted(evs)
	if ac == nil {
		t.Fatalf("expected action_completed in export")
	}
	tr := findByType(evs, "trade_agreement")
	if tr == nil {
		t.Fatalf("expected trade_agreement in export")
	}
	if tr.Ts <= ac.Ts {
		t.Fatalf("expected trade_agreement after action_completed (ts), got ac.ts=%d tr.ts=%d", ac.Ts, tr.Ts)
	}
	if len(tr.Actors) != 2 || tr.Actors[0] == tr.Actors[1] {
		t.Fatalf("expected 2 distinct actors, got %#v", tr.Actors)
	}
	if _, ok := num(tr.Delta, "food"); !ok {
		t.Fatalf("expected delta.food in trade_agreement, got %#v", tr.Delta)
	}
	if !hasTraceCause(*tr, ac.EventID) {
		t.Fatalf("expected trade_agreement trace links to action_completed %q", ac.EventID)
	}
}

