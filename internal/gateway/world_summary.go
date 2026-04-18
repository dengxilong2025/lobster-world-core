package gateway

import (
	"fmt"
	"strings"

	"lobster-world-core/internal/sim"
)

type WorldSummary struct {
	Stage   string   `json:"stage"`
	Summary []string `json:"summary"`
	State   map[string]any `json:"state"`
}

func deriveWorldSummary(st sim.Status) WorldSummary {
	stage := "萌芽"
	switch {
	case st.State.Conflict >= 70:
		stage = "战乱"
	case st.State.Food <= 10:
		stage = "饥荒"
	case st.State.Order <= 20:
		stage = "失序"
	case st.State.Knowledge >= 200:
		stage = "启蒙"
	case st.State.Population >= 150 && st.State.Food >= 60:
		stage = "扩张"
	}

	bullets := make([]string, 0, 4)
	bullets = append(bullets, fmt.Sprintf("阶段：%s", stage))
	if st.State.Food <= 20 {
		bullets = append(bullets, "风险：食物紧缺")
	}
	if st.State.Conflict >= 60 {
		bullets = append(bullets, "风险：冲突高企")
	}
	if st.State.Trust <= 25 {
		bullets = append(bullets, "风险：互不信任蔓延")
	}
	if len(bullets) < 2 {
		bullets = append(bullets, "建议：继续提交意图推动世界叙事")
	}

	// Deduplicate (just in case).
	seen := map[string]struct{}{}
	out := make([]string, 0, len(bullets))
	for _, b := range bullets {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		if _, ok := seen[b]; ok {
			continue
		}
		seen[b] = struct{}{}
		out = append(out, b)
	}

	return WorldSummary{
		Stage:   stage,
		Summary: out,
		State: map[string]any{
			"food":       st.State.Food,
			"population": st.State.Population,
			"order":      st.State.Order,
			"trust":      st.State.Trust,
			"knowledge":  st.State.Knowledge,
			"conflict":   st.State.Conflict,
			"tick":       st.Tick,
		},
	}
}

