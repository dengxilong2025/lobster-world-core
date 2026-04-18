package gateway

import (
	"sort"
	"strings"

	"lobster-world-core/internal/events/spec"
	"lobster-world-core/internal/events/store"
)

// pickRecentFromTrace extracts up to N distinct "meaningful" narratives from trace causes.
// It fetches cause events by ID and prefers intent_accepted/shock/betrayal over generic actions.
func pickRecentFromTrace(es store.EventStore, worldID string, target spec.Event, n int) []string {
	if n <= 0 {
		n = 2
	}
	type cand struct {
		narr string
		prio int
	}
	cands := make([]cand, 0, len(target.Trace))

	for _, tl := range target.Trace {
		if strings.TrimSpace(tl.CauseEventID) == "" {
			continue
		}
		ce, ok, err := es.GetByID(worldID, tl.CauseEventID)
		if err != nil || !ok {
			continue
		}
		narr := strings.TrimSpace(ce.Narrative)
		if narr == "" {
			continue
		}
		prio := 50
		switch ce.Type {
		case "intent_accepted":
			prio = 0
		case "betrayal":
			prio = 5
		case "shock_started", "shock_warning", "shock_ended":
			prio = 8
		case "world_evolved":
			prio = 15
		case "action_started", "action_completed":
			prio = 80
		default:
			prio = 40
		}
		cands = append(cands, cand{narr: narr, prio: prio})
	}

	sort.Slice(cands, func(i, j int) bool {
		if cands[i].prio != cands[j].prio {
			return cands[i].prio < cands[j].prio
		}
		return cands[i].narr < cands[j].narr
	})

	out := make([]string, 0, n)
	seen := map[string]struct{}{}
	for _, c := range cands {
		if len(out) >= n {
			break
		}
		if _, ok := seen[c.narr]; ok {
			continue
		}
		seen[c.narr] = struct{}{}
		out = append(out, c.narr)
	}
	return out
}
