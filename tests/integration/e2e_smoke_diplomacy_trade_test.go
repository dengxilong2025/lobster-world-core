package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lobster-world-core/internal/gateway"
)

func TestE2ESmoke_DiplomacyTradeIntent_ToEvents_ToExport(t *testing.T) {
	t.Parallel()

	app := gateway.NewAppWithOptions(gateway.AppOptions{
		TickInterval: 5 * time.Millisecond,
		Seed:         123,
	})
	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)
	t.Cleanup(func() { app.Stop() })

	worldID := "w_e2e_smoke"

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

	// 1) Build conflict risk deterministically so home suggestions include diplomacy guidance.
	for i := 0; i < 10; i++ {
		postIntent("背叛：挑起冲突")
	}
	time.Sleep(300 * time.Millisecond)

	// 2) home hint should mention diplomacy keywords + expected event types.
	hr, err := http.Get(s.URL + "/api/v0/spectator/home?world_id=" + worldID)
	if err != nil {
		t.Fatalf("GET home: %v", err)
	}
	defer hr.Body.Close()
	if hr.StatusCode != http.StatusOK {
		t.Fatalf("home status=%d", hr.StatusCode)
	}
	hb, _ := io.ReadAll(hr.Body)
	hs := string(hb)
	if !strings.Contains(hs, "建议：") {
		t.Fatalf("expected hints in home, got=%s", hs)
	}
	if !(strings.Contains(hs, "停战") || strings.Contains(hs, "谈判") || strings.Contains(hs, "条约") || strings.Contains(hs, "结盟")) {
		t.Fatalf("expected diplomacy keywords in home hints, got=%s", hs)
	}
	if !(strings.Contains(hs, "alliance_formed") || strings.Contains(hs, "treaty_signed")) {
		t.Fatalf("expected expected-event types in home hints, got=%s", hs)
	}

	// 3) Produce story events deterministically.
	postIntent("结盟：达成联盟")
	postIntent("条约：签署停战条约")
	postIntent("贸易：开通商路")
	time.Sleep(300 * time.Millisecond)

	// 4) export should contain story event types.
	er, err := http.Get(s.URL + "/api/v0/replay/export?world_id=" + worldID + "&limit=5000")
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	defer er.Body.Close()
	if er.StatusCode != http.StatusOK {
		t.Fatalf("export status=%d", er.StatusCode)
	}
	eb, _ := io.ReadAll(er.Body)
	es := string(eb)
	if !strings.Contains(es, "\"type\":\"alliance_formed\"") {
		t.Fatalf("missing alliance_formed in export")
	}
	if !strings.Contains(es, "\"type\":\"treaty_signed\"") {
		t.Fatalf("missing treaty_signed in export")
	}
	if !strings.Contains(es, "\"type\":\"trade_agreement\"") {
		t.Fatalf("missing trade_agreement in export")
	}
}

