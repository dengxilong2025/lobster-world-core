package sim

import "testing"

func TestWorldStateApplyDelta_ClampsToRange(t *testing.T) {
	s := WorldState{
		Food:       50,
		Population: 50,
		Order:      50,
		Trust:      50,
		Knowledge:  10,
		Conflict:   10,
	}

	// Push beyond both ends.
	s.ApplyDelta(map[string]any{
		"food":       int64(9999),
		"population": int64(-9999),
		"order":      int64(-9999),
		"trust":      int64(9999),
		"knowledge":  int64(9999),
		"conflict":   int64(-9999),
	})

	// Expect clamps:
	// - most indicators in [0,100]
	// - knowledge in [0,1000]
	if s.Food != 100 {
		t.Fatalf("food clamp: got %d", s.Food)
	}
	if s.Population != 0 {
		t.Fatalf("population clamp: got %d", s.Population)
	}
	if s.Order != 0 {
		t.Fatalf("order clamp: got %d", s.Order)
	}
	if s.Trust != 100 {
		t.Fatalf("trust clamp: got %d", s.Trust)
	}
	if s.Conflict != 0 {
		t.Fatalf("conflict clamp: got %d", s.Conflict)
	}
	if s.Knowledge != 1000 {
		t.Fatalf("knowledge clamp: got %d", s.Knowledge)
	}
}

