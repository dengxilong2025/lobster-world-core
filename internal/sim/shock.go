package sim

import (
	"fmt"
	"hash/fnv"
	"math/rand"

	"lobster-world-core/internal/events/spec"
)

// ShockConfig defines the 72h shock scheduling behavior (v0: configurable, half-random).
//
// Terms:
// - EpochTicks: one "show cycle" length in ticks. (72h in production; small in tests)
// - WarningOffset: emit shock_warning at (epochStartTick - WarningOffset)
// - DurationTicks: shock ends at (epochStartTick + DurationTicks)
// - CooldownTicks: do not repeat the same shock key within this tick distance.
type ShockConfig struct {
	EpochTicks    int64
	WarningOffset int64
	DurationTicks int64
	CooldownTicks int64

	Candidates []ShockCandidate
}

type ShockCandidate struct {
	Key             string
	Weight          int64
	WarningNarrative string
	StartedNarrative string
	EndedNarrative   string
}

type shockScheduler struct {
	cfg ShockConfig

	lastKey      string
	lastStartTick int64

	// epochStartTick -> chosen key
	epochChoice map[int64]ShockCandidate
}

func newShockScheduler(cfg ShockConfig) *shockScheduler {
	if cfg.EpochTicks <= 0 {
		cfg.EpochTicks = 720 // placeholder; production will use 72h/tickInterval
	}
	if cfg.WarningOffset < 0 {
		cfg.WarningOffset = 0
	}
	if cfg.DurationTicks <= 0 {
		cfg.DurationTicks = 3
	}
	if cfg.CooldownTicks < 0 {
		cfg.CooldownTicks = 0
	}
	return &shockScheduler{
		cfg:         cfg,
		epochChoice: map[int64]ShockCandidate{},
	}
}

func (s *shockScheduler) step(worldID string, tick int64) []spec.Event {
	if len(s.cfg.Candidates) == 0 {
		return nil
	}

	// Epoch start ticks are 1-based (first start is at EpochTicks, not 0),
	// so that we don't immediately emit "ended" events at tick=DurationTicks.
	epochStart := currentEpochStart(tick, s.cfg.EpochTicks)
	chosen := s.ensureChoice(worldID, epochStart)

	var out []spec.Event
	if s.cfg.WarningOffset > 0 && tick == epochStart-s.cfg.WarningOffset && tick >= 0 {
		out = append(out, makeShockEvent(worldID, tick, "shock_warning", chosen.Key, chosen.WarningNarrative))
	}
	if tick == epochStart {
		out = append(out, makeShockEvent(worldID, tick, "shock_started", chosen.Key, chosen.StartedNarrative))
		s.lastKey = chosen.Key
		s.lastStartTick = epochStart
	}
	if tick == epochStart+s.cfg.DurationTicks {
		out = append(out, makeShockEvent(worldID, tick, "shock_ended", chosen.Key, chosen.EndedNarrative))
	}
	return out
}

func currentEpochStart(tick, epochTicks int64) int64 {
	if epochTicks <= 0 {
		return 0
	}
	if tick <= 0 {
		return epochTicks
	}
	if tick < epochTicks {
		return epochTicks
	}
	return (tick / epochTicks) * epochTicks
}

func (s *shockScheduler) ensureChoice(worldID string, epochStart int64) ShockCandidate {
	if c, ok := s.epochChoice[epochStart]; ok {
		return c
	}

	r := rand.New(rand.NewSource(seedFor(worldID, epochStart)))
	choice := weightedPick(r, s.cfg.Candidates)

	// cooldown: if repeating last key too soon, try to pick another.
	if s.cfg.CooldownTicks > 0 && s.lastKey != "" && choice.Key == s.lastKey && epochStart-s.lastStartTick < s.cfg.CooldownTicks {
		alts := make([]ShockCandidate, 0, len(s.cfg.Candidates)-1)
		for _, c := range s.cfg.Candidates {
			if c.Key != s.lastKey {
				alts = append(alts, c)
			}
		}
		if len(alts) > 0 {
			choice = weightedPick(r, alts)
		}
	}

	s.epochChoice[epochStart] = choice
	return choice
}

func weightedPick(r *rand.Rand, items []ShockCandidate) ShockCandidate {
	var total int64
	for _, it := range items {
		if it.Weight > 0 {
			total += it.Weight
		}
	}
	if total <= 0 {
		return items[0]
	}
	n := r.Int63n(total)
	var acc int64
	for _, it := range items {
		if it.Weight <= 0 {
			continue
		}
		acc += it.Weight
		if n < acc {
			return it
		}
	}
	return items[len(items)-1]
}

func seedFor(worldID string, epochStart int64) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(worldID))
	_, _ = h.Write([]byte(fmt.Sprintf("|%d", epochStart)))
	return int64(h.Sum64())
}

func makeShockEvent(worldID string, tick int64, typ string, shockKey string, narrative string) spec.Event {
	if narrative == "" {
		narrative = typ
	}
	return spec.Event{
		SchemaVersion: 1,
		EventID:       fmt.Sprintf("evt_%s_shock_%d_%s", worldID, tick, typ),
		Ts:            0, // will be overwritten by caller if desired
		WorldID:       worldID,
		Scope:         "world",
		Type:          typ,
		Actors:        []string{"world"},
		Narrative:     narrative,
		Tick:          tick,
		Meta:          map[string]any{"shock_key": shockKey},
	}
}
