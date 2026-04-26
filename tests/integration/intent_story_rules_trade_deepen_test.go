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

type exportEventTD struct {
	spec.Event
	ExportSchemaVersion int `json:"export_schema_version"`
}

func readExportTD(t *testing.T, baseURL, worldID string) []exportEventTD {
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
	out := []exportEventTD{}
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var ev exportEventTD
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

func findByTypeTD(evs []exportEventTD, typ string) *exportEventTD {
	for i := range evs {
		if evs[i].Type == typ {
			return &evs[i]
		}
	}
	return nil
}

func findActionCompletedIDsTD(evs []exportEventTD) map[string]struct{} {
	ids := map[string]struct{}{}
	for i := range evs {
		if evs[i].Type == "action_completed" {
			ids[evs[i].EventID] = struct{}{}
		}
	}
	return ids
}

func hasTraceCauseInSetTD(e exportEventTD, ids map[string]struct{}) bool {
	for _, tl := range e.Trace {
		if _, ok := ids[tl.CauseEventID]; ok {
			return true
		}
	}
	return false
}

func numTD(m map[string]any, k string) (int64, bool) {
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

func TestStoryRules_TradeDeepen_BoomAndDispute(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		Seed:         789,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_story_trade_deepen"

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

	postIntent("开放贸易：市场繁荣")
	postIntent("封锁：加税关税")
	time.Sleep(400 * time.Millisecond)

	evs := readExportTD(t, s.URL, worldID)
	acIDs := findActionCompletedIDsTD(evs)
	if len(acIDs) == 0 {
		t.Fatalf("missing action_completed in export")
	}

	boom := findByTypeTD(evs, "market_boom")
	if boom == nil {
		t.Fatalf("missing market_boom in export")
	}
	if len(boom.Actors) != 2 || boom.Actors[0] == boom.Actors[1] {
		t.Fatalf("market_boom actors invalid: %v", boom.Actors)
	}
	if !hasTraceCauseInSetTD(*boom, acIDs) {
		t.Fatalf("market_boom should trace some action_completed")
	}
	if v, ok := numTD(boom.Delta, "food"); !ok || v <= 0 {
		t.Fatalf("market_boom expected food>0 delta=%v", boom.Delta)
	}

	dis := findByTypeTD(evs, "trade_dispute")
	if dis == nil {
		t.Fatalf("missing trade_dispute in export")
	}
	if len(dis.Actors) != 2 || dis.Actors[0] == dis.Actors[1] {
		t.Fatalf("trade_dispute actors invalid: %v", dis.Actors)
	}
	if !hasTraceCauseInSetTD(*dis, acIDs) {
		t.Fatalf("trade_dispute should trace some action_completed")
	}
	if v, ok := numTD(dis.Delta, "conflict"); !ok || v <= 0 {
		t.Fatalf("trade_dispute expected conflict>0 delta=%v", dis.Delta)
	}
	if v, ok := numTD(dis.Delta, "trust"); !ok || v >= 0 {
		t.Fatalf("trade_dispute expected trust<0 delta=%v", dis.Delta)
	}

	// Home hints should mention at least one of the expected trade deepen event types.
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
	if !(strings.Contains(hs, "market_boom") || strings.Contains(hs, "trade_dispute")) {
		t.Fatalf("expected home hints mention market_boom/trade_dispute, got=%s", hs)
	}
}
