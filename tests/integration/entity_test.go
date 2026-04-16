package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/gateway"
)

func TestSpectatorEntity_ReturnsRelationsAndRecentEvents(t *testing.T) {
	t.Parallel()

	app := gateway.NewApp()
	_ = app.EventStore.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_rel_1",
		Ts:            10,
		WorldID:       "w1",
		Scope:         "world",
		Type:          "alliance_formed",
		Actors:        []string{"nation_a", "nation_b"},
		Narrative:     "血盟成立：A与B结盟",
	})
	_ = app.EventStore.Append(spec.Event{
		SchemaVersion: 1,
		EventID:       "evt_ent_1",
		Ts:            20,
		WorldID:       "w1",
		Scope:         "entity",
		EntityID:      "nation_a",
		Type:          "skill_gained",
		Actors:        []string{"nation_a"},
		Narrative:     "A获得《驯化》",
	})

	s := httptest.NewServer(app.Handler)
	t.Cleanup(s.Close)

	resp, err := http.Get(s.URL + "/api/v0/spectator/entity?world_id=w1&entity_id=nation_a")
	if err != nil {
		t.Fatalf("GET /spectator/entity: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", out)
	}

	rels, ok := out["relations"].([]any)
	if !ok || len(rels) == 0 {
		t.Fatalf("expected relations, got %#v", out["relations"])
	}
	events, ok := out["recent_events"].([]any)
	if !ok || len(events) == 0 {
		t.Fatalf("expected recent_events, got %#v", out["recent_events"])
	}
}

