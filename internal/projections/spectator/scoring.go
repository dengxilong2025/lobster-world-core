package spectator

// scoreEvent assigns a "hotness" score to an event based on type and recency.
//
// v0 design goals:
// - Deterministic (no floating point required)
// - Simple to reason about
// - Easy to tune via weights + half-life
//
// score = weight(type) / (1 + age/halfLife)
func scoreEventTicks(eType string, ageTicks int64, halfLifeTicks int64) int64 {
	weight := int64(10)
	switch eType {
	case "betrayal":
		weight = 120
	case "war_started":
		weight = 110
	case "battle_resolved":
		weight = 95
	case "shock_started":
		weight = 90
	case "shock_warning":
		weight = 70
	case "skill_gained":
		weight = 60
	case "adoption_confirmed", "adoption_revoked":
		weight = 20
	case "intent_accepted", "action_started", "action_completed":
		weight = 15
	}

	if halfLifeTicks <= 0 {
		halfLifeTicks = 360
	}
	if ageTicks < 0 {
		ageTicks = 0
	}
	return weight / (1 + (ageTicks / halfLifeTicks))
}

// scoreEventSec is a fallback for events that don't have tick information.
// Kept for compatibility with legacy/manual events that only set ts.
func scoreEventSec(eType string, ageSec int64, halfLifeSec int64) int64 {
	if halfLifeSec <= 0 {
		halfLifeSec = 1800
	}
	if ageSec < 0 {
		ageSec = 0
	}
	// Reuse same weight table by mapping to tick scoring.
	// (weight logic is independent; only decay differs)
	return scoreEventTicks(eType, ageSec, halfLifeSec)
}
