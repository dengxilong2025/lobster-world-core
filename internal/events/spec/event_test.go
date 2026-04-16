package spec

import "testing"

func TestEventValidate_RejectsMissingFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		e    Event
	}{
		{
			name: "missing world_id",
			e: Event{
				SchemaVersion: 1,
				EventID:       "evt_1",
				Ts:            1710000000,
				WorldID:       "",
				Scope:         "world",
				Type:          "shock_warning",
				Actors:        []string{"lobster_1"},
				Narrative:     "天象异常：裂冬指数上升",
			},
		},
		{
			name: "missing type",
			e: Event{
				SchemaVersion: 1,
				EventID:       "evt_1",
				Ts:            1710000000,
				WorldID:       "w_stone_age_1",
				Scope:         "world",
				Type:          "",
				Actors:        []string{"lobster_1"},
				Narrative:     "天象异常：裂冬指数上升",
			},
		},
		{
			name: "missing narrative",
			e: Event{
				SchemaVersion: 1,
				EventID:       "evt_1",
				Ts:            1710000000,
				WorldID:       "w_stone_age_1",
				Scope:         "world",
				Type:          "shock_warning",
				Actors:        []string{"lobster_1"},
				Narrative:     "",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.e.Validate(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

