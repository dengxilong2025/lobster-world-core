package sim

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"

	"lobster-world-core/internal/events/spec"
)

// defaultStoryActorPool is a small fixed pool for v0.3-M1 story rules.
// It MUST be deterministic and stable so replay/export remains comparable across runs.
var defaultStoryActorPool = []string{"nation_a", "nation_b", "nation_c", "nation_d", "nation_e", "nation_f"}

func seedForIntent(worldSeed int64, tick int64, intentID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("%d|%d|%s", worldSeed, tick, intentID)))
	return int64(h.Sum64())
}

func pick2DistinctForIntent(worldSeed int64, tick int64, intentID string, pool []string) (string, string, bool) {
	if len(pool) < 2 {
		return "", "", false
	}
	r := rand.New(rand.NewSource(seedForIntent(worldSeed, tick, intentID)))
	i := r.Intn(len(pool))
	j := r.Intn(len(pool) - 1)
	if j >= i {
		j++
	}
	return pool[i], pool[j], true
}

type storyEventSpec struct {
	typ       string
	narrative string
	delta     map[string]any
}

func intentStorySpec(goal string) (storyEventSpec, bool) {
	g := strings.TrimSpace(goal)
	if g == "" {
		return storyEventSpec{}, false
	}

	// Precedence: diplomacy > trade (avoid emitting multiple extra events per intent in v0.3-M1).
	if strings.Contains(g, "结盟") || strings.Contains(g, "联盟") {
		return storyEventSpec{
			typ:       "alliance_formed",
			narrative: "外交突破：达成同盟（目标：" + g + "）",
			delta:     map[string]any{"trust": int64(8), "order": int64(2), "conflict": int64(-3)},
		}, true
	}
	if strings.Contains(g, "条约") || strings.Contains(g, "停战") || strings.Contains(g, "谈判") {
		return storyEventSpec{
			typ:       "treaty_signed",
			narrative: "外交突破：签署条约（目标：" + g + "）",
			delta:     map[string]any{"trust": int64(6), "order": int64(2), "conflict": int64(-2)},
		}, true
	}
	if strings.Contains(g, "贸易") || strings.Contains(g, "集市") || strings.Contains(g, "交换") || strings.Contains(g, "商路") {
		return storyEventSpec{
			typ:       "trade_agreement",
			narrative: "贸易达成：开通商路（目标：" + g + "）",
			delta:     map[string]any{"food": int64(5), "trust": int64(3), "knowledge": int64(1), "conflict": int64(-1)},
		}, true
	}
	return storyEventSpec{}, false
}

func buildStoryWorldEvent(worldID string, tick int64, worldSeed int64, intentID string, goal string, acceptedEventID string, actionCompletedEventID string) (spec.Event, bool) {
	sp, ok := intentStorySpec(goal)
	if !ok {
		return spec.Event{}, false
	}
	a, b, ok := pick2DistinctForIntent(worldSeed, tick, intentID, defaultStoryActorPool)
	if !ok {
		return spec.Event{}, false
	}

	// Note: caller will fill EventID/Ts via w.newEventLocked(), but we keep this struct for
	// clarity and to ensure all fields are deterministic.
	ev := spec.Event{
		SchemaVersion: 1,
		EventID:       "",
		Ts:            0,
		WorldID:       worldID,
		Scope:         "world",
		Type:          sp.typ,
		Actors:        []string{a, b},
		Narrative:     sp.narrative,
		Tick:          tick,
		Delta:         sp.delta,
		Trace: []spec.TraceLink{
			{CauseEventID: actionCompletedEventID, Note: "从意图执行结果导出剧本事件"},
		},
	}
	if acceptedEventID != "" {
		ev.Trace = append(ev.Trace, spec.TraceLink{CauseEventID: acceptedEventID, Note: "回溯意图来源：" + strings.TrimSpace(goal)})
	}
	return ev, true
}

