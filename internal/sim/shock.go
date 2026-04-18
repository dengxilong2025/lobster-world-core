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

func pick2Distinct(worldSeed int64, epochStart int64, pool []string) (string, string, bool) {
	if len(pool) < 2 {
		return "", "", false
	}
	r := rand.New(rand.NewSource(seedFor(worldSeed, epochStart) + 7))
	i := r.Intn(len(pool))
	j := r.Intn(len(pool) - 1)
	if j >= i {
		j++
	}
	return pool[i], pool[j], true
}

func makeBetrayalEvent(worldID string, tick int64, shockKey, causeEventID, a, b string) spec.Event {
	return spec.Event{
		SchemaVersion: 1,
		EventID:       fmt.Sprintf("evt_%s_shock_%d_betrayal_%s_%s", worldID, tick, a, b),
		Ts:            0,
		WorldID:       worldID,
		Scope:         "world",
		Type:          "betrayal",
		Actors:        []string{a, b},
		Narrative:     fmt.Sprintf("冲击撕裂盟约：%s 背叛 %s", a, b),
		Tick:          tick,
		Delta: map[string]any{
			"trust":    int64(-7),
			"order":    int64(-5),
			"conflict": int64(+6),
		},
		Trace: []spec.TraceLink{
			{CauseEventID: causeEventID, Note: "冲击期资源紧缩导致同盟破裂"},
			{CauseEventID: causeEventID, Note: "恐慌蔓延引发互不信任，背刺成为最短路径"},
		},
		Meta:          map[string]any{"shock_key": shockKey},
	}
}

type ShockCandidate struct {
	Key             string
	Weight          int64
	WarningNarrative string
	StartedNarrative string
	EndedNarrative   string

	// Optional deltas to apply to world state when each phase happens (v0 "爽感delta").
	WarningDelta map[string]int64
	StartedDelta map[string]int64
	EndedDelta   map[string]int64

	// Optional actor pool for producing relationship drama during the shock.
	// When provided (len>=2), shock_started will also emit one betrayal event between two distinct actors.
	ActorsPool []string
}

type shockScheduler struct {
	cfg ShockConfig
	worldSeed int64
	maxEpochChoices int
	epochOrder      []int64

	lastKey      string
	lastStartTick int64

	// epochStartTick -> chosen key
	epochChoice map[int64]ShockCandidate
}

func newShockScheduler(cfg ShockConfig, worldSeed int64, maxEpochChoices int) *shockScheduler {
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
	if maxEpochChoices <= 0 {
		maxEpochChoices = 256
	}
	return &shockScheduler{
		cfg:         cfg,
		worldSeed:   worldSeed,
		maxEpochChoices: maxEpochChoices,
		epochChoice: map[int64]ShockCandidate{},
		epochOrder:  []int64{},
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
		out = append(out, makeShockEvent(worldID, tick, "shock_warning", chosen.Key, chosen.WarningNarrative, chosen.WarningDelta))
	}
	if tick == epochStart {
		started := makeShockEvent(worldID, tick, "shock_started", chosen.Key, chosen.StartedNarrative, chosen.StartedDelta)
		out = append(out, started)
		// Relationship drama (v0): inject one betrayal between two actors from the configured pool.
		if len(chosen.ActorsPool) >= 2 {
			a, b, ok := pick2Distinct(s.worldSeed, epochStart, chosen.ActorsPool)
			if ok {
				out = append(out, makeBetrayalEvent(worldID, tick, chosen.Key, started.EventID, a, b))
			}
		}
		s.lastKey = chosen.Key
		s.lastStartTick = epochStart
	}
	if tick == epochStart+s.cfg.DurationTicks {
		out = append(out, makeShockEvent(worldID, tick, "shock_ended", chosen.Key, chosen.EndedNarrative, chosen.EndedDelta))
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

	r := rand.New(rand.NewSource(seedFor(s.worldSeed, epochStart)))
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
	s.epochOrder = append(s.epochOrder, epochStart)
	// Bound memory: keep only the most recent N epoch choices.
	if len(s.epochOrder) > s.maxEpochChoices {
		excess := len(s.epochOrder) - s.maxEpochChoices
		for i := 0; i < excess; i++ {
			old := s.epochOrder[i]
			delete(s.epochChoice, old)
		}
		s.epochOrder = s.epochOrder[excess:]
	}
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

func seedFor(worldSeed int64, epochStart int64) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("%d", worldSeed)))
	_, _ = h.Write([]byte(fmt.Sprintf("|%d", epochStart)))
	return int64(h.Sum64())
}

func makeShockEvent(worldID string, tick int64, typ string, shockKey string, narrative string, delta map[string]int64) spec.Event {
	if narrative == "" {
		narrative = typ
	}
	var d map[string]any
	if len(delta) > 0 {
		d = map[string]any{}
		for k, v := range delta {
			d[k] = v
		}
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
		Delta:         d,
		Meta:          map[string]any{"shock_key": shockKey},
	}
}
