package spec

import "fmt"

// Event is the canonical event object defined by docs/15_event_spec_v0.md (schema_version=1).
//
// This object is transport-agnostic: stored in an append-only log, pulled via HTTP, pushed via SSE/WS,
// and exported as NDJSON for replay/debugging.
type Event struct {
	SchemaVersion int      `json:"schema_version"`
	EventID       string   `json:"event_id"`
	Ts            int64    `json:"ts"`
	WorldID       string   `json:"world_id"`
	Scope         string   `json:"scope"` // world|entity
	Type          string   `json:"type"`
	Actors        []string `json:"actors"`
	Narrative     string   `json:"narrative"`

	// Optional fields for v0 will be added later (tick/entity_id/delta/trace/meta).
}

// Validate checks minimal v0 invariants (MUST fields). v0 keeps this intentionally small.
func (e Event) Validate() error {
	if e.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1")
	}
	if e.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	if e.Ts <= 0 {
		return fmt.Errorf("ts is required")
	}
	if e.WorldID == "" {
		return fmt.Errorf("world_id is required")
	}
	if e.Scope != "world" && e.Scope != "entity" {
		return fmt.Errorf("scope must be 'world' or 'entity'")
	}
	if e.Type == "" {
		return fmt.Errorf("type is required")
	}
	if len(e.Actors) == 0 {
		return fmt.Errorf("actors is required")
	}
	if e.Narrative == "" {
		return fmt.Errorf("narrative is required")
	}
	return nil
}
