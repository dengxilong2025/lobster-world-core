package gateway

import (
	"testing"

	"lobster-world-core/internal/sim"
)

func TestDeriveWorldSummary_StagePriority_TableDriven(t *testing.T) {
	t.Parallel()

	// Priority encoded in deriveWorldSummary:
	// 战乱(conflict>=70) > 饥荒(food<=10) > 失序(order<=20) > 启蒙(knowledge>=200) > 扩张(pop>=150 && food>=60) > 萌芽(default)
	tests := []struct {
		name  string
		state sim.WorldState
		want  string
	}{
		{
			name:  "default",
			state: sim.WorldState{Food: 50, Population: 50, Order: 50, Trust: 50, Knowledge: 0, Conflict: 0},
			want:  "萌芽",
		},
		{
			name:  "expansion_when_pop_high_and_food_high",
			state: sim.WorldState{Food: 60, Population: 150, Order: 50, Trust: 50, Knowledge: 0, Conflict: 0},
			want:  "扩张",
		},
		{
			name:  "enlightenment_when_knowledge_high",
			state: sim.WorldState{Food: 60, Population: 150, Order: 50, Trust: 50, Knowledge: 200, Conflict: 0},
			want:  "启蒙",
		},
		{
			name:  "disorder_when_order_low",
			state: sim.WorldState{Food: 60, Population: 150, Order: 20, Trust: 50, Knowledge: 200, Conflict: 0},
			want:  "失序",
		},
		{
			name:  "famine_when_food_very_low",
			state: sim.WorldState{Food: 10, Population: 150, Order: 20, Trust: 50, Knowledge: 200, Conflict: 0},
			want:  "饥荒",
		},
		{
			name:  "war_has_highest_priority_over_famine_and_disorder",
			state: sim.WorldState{Food: 10, Population: 150, Order: 20, Trust: 50, Knowledge: 200, Conflict: 70},
			want:  "战乱",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ws := deriveWorldSummary(sim.Status{Tick: 1, State: tt.state}, nil)
			if ws.Stage != tt.want {
				t.Fatalf("stage=%q want=%q (state=%+v)", ws.Stage, tt.want, tt.state)
			}
		})
	}
}

