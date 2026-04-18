package sim

// WorldState is the minimal set of abstract indicators for v0.
// Values are integers for determinism and easy reasoning.
type WorldState struct {
	Food       int64 `json:"food"`
	Population int64 `json:"population"`
	Order      int64 `json:"order"`
	Trust      int64 `json:"trust"`
	Knowledge  int64 `json:"knowledge"`
	Conflict   int64 `json:"conflict"`
}

func (s *WorldState) ApplyDelta(delta map[string]any) {
	if delta == nil {
		return
	}
	for k, v := range delta {
		d, ok := asInt64(v)
		if !ok {
			continue
		}
		switch k {
		case "food":
			s.Food += d
		case "population":
			s.Population += d
		case "order":
			s.Order += d
		case "trust":
			s.Trust += d
		case "knowledge":
			s.Knowledge += d
		case "conflict":
			s.Conflict += d
		}
	}

	// Clamp v0 indicators to keep the world state meaningful over long runs.
	// This is a safety invariant; narrative events on clamp can be added later.
	s.Food = clamp(s.Food, 0, 100)
	s.Population = clamp(s.Population, 0, 100)
	s.Order = clamp(s.Order, 0, 100)
	s.Trust = clamp(s.Trust, 0, 100)
	s.Conflict = clamp(s.Conflict, 0, 100)
	s.Knowledge = clamp(s.Knowledge, 0, 1000)
}

func clamp(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func asInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case float32:
		return int64(t), true
	case float64:
		return int64(t), true
	default:
		return 0, false
	}
}
