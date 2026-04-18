package sim

import "testing"

func TestShockScheduler_EpochChoiceIsBounded(t *testing.T) {
	t.Parallel()

	cfg := ShockConfig{
		EpochTicks:    6,
		WarningOffset: 1,
		DurationTicks: 2,
		CooldownTicks: 6,
		Candidates: []ShockCandidate{
			{Key: "a", Weight: 1, WarningNarrative: "w", StartedNarrative: "s", EndedNarrative: "e", ActorsPool: []string{"x", "y"}},
			{Key: "b", Weight: 1, WarningNarrative: "w2", StartedNarrative: "s2", EndedNarrative: "e2", ActorsPool: []string{"x", "y"}},
		},
	}

	s := newShockScheduler(cfg, 123, 3)
	// Trigger choices for 5 epochs.
	for _, epochStart := range []int64{0, 6, 12, 18, 24} {
		_ = s.ensureChoice("w", epochStart)
	}
	if got := len(s.epochChoice); got != 3 {
		t.Fatalf("expected bounded epochChoice size=3, got %d", got)
	}
	if _, ok := s.epochChoice[0]; ok {
		t.Fatalf("expected oldest epochStart evicted")
	}
	if _, ok := s.epochChoice[6]; ok {
		t.Fatalf("expected second-oldest epochStart evicted")
	}
	if _, ok := s.epochChoice[12]; !ok {
		t.Fatalf("expected recent epochStart retained")
	}
}

