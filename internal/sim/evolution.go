package sim

import "strings"

// evolveLocked applies simple deterministic "natural dynamics" each tick.
// Returns (typ, narrative, delta, okToEmit).
func (w *world) evolveLocked() (string, string, map[string]any, bool) {
	var notes []string
	delta := map[string]any{}

	// Food scarcity -> population decline + conflict rise.
	if w.state.Food <= 0 {
		notes = append(notes, "饥荒蔓延")
		add(delta, "population", -2)
		add(delta, "trust", -2)
		add(delta, "conflict", +2)
	} else if w.state.Food < 20 {
		notes = append(notes, "食物紧缺")
		add(delta, "population", -1)
		add(delta, "conflict", +1)
	}

	// High conflict erodes trust & order.
	if w.state.Conflict > 70 {
		notes = append(notes, "冲突升温")
		add(delta, "trust", -1)
		add(delta, "order", -1)
	}

	// Low trust tends to increase conflict.
	if w.state.Trust < 20 {
		notes = append(notes, "互不信任")
		add(delta, "conflict", +1)
	}

	// Stable abundance -> gentle growth.
	if w.state.Food > 80 && w.state.Order > 40 && w.state.Conflict < 30 {
		notes = append(notes, "丰收带来繁荣")
		add(delta, "population", +1)
		add(delta, "trust", +1)
	}

	if len(delta) == 0 {
		return "", "", nil, false
	}
	narr := strings.Join(notes, "；")
	if narr == "" {
		narr = "世界自发演化"
	}
	return "world_evolved", narr, delta, true
}

func add(m map[string]any, k string, v int64) {
	if m == nil {
		return
	}
	if cur, ok := m[k]; ok {
		if n, ok2 := asInt64(cur); ok2 {
			m[k] = n + v
			return
		}
	}
	m[k] = v
}

