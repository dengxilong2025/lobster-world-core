package sim

import "strings"

// v0 intent ruleset:
// Minimal deterministic mapping from intent goal text -> world state delta.
// This is intentionally simple and will evolve into a richer rules engine (P2/P3).
func intentDelta(goal string) map[string]any {
	g := strings.TrimSpace(goal)
	if g == "" {
		return map[string]any{"knowledge": int64(1)}
	}

	// Resource gathering
	if strings.Contains(g, "狩猎") || strings.Contains(g, "捕猎") || strings.Contains(g, "打猎") {
		return map[string]any{"food": int64(8), "knowledge": int64(1)}
	}
	if strings.Contains(g, "采集") || strings.Contains(g, "种植") || strings.Contains(g, "耕作") {
		return map[string]any{"food": int64(5), "order": int64(1)}
	}

	// Social dynamics
	if strings.Contains(g, "结盟") || strings.Contains(g, "联盟") {
		return map[string]any{"trust": int64(4), "order": int64(1), "conflict": int64(-1)}
	}
	if strings.Contains(g, "背叛") {
		return map[string]any{"trust": int64(-6), "conflict": int64(6), "order": int64(-2)}
	}

	// Exploration / default progress
	return map[string]any{"knowledge": int64(1)}
}

func explainIntentRule(goal string) string {
	g := strings.TrimSpace(goal)
	if g == "" {
		return "空目标 -> 知识+1"
	}
	if strings.Contains(g, "狩猎") || strings.Contains(g, "捕猎") || strings.Contains(g, "打猎") {
		return "狩猎 -> 食物+8，知识+1"
	}
	if strings.Contains(g, "采集") || strings.Contains(g, "种植") || strings.Contains(g, "耕作") {
		return "耕作/采集 -> 食物+5，秩序+1"
	}
	if strings.Contains(g, "结盟") || strings.Contains(g, "联盟") {
		return "结盟 -> 信任+4，秩序+1，冲突-1"
	}
	if strings.Contains(g, "背叛") {
		return "背叛 -> 信任-6，冲突+6，秩序-2"
	}
	return "探索/推进 -> 知识+1"
}
