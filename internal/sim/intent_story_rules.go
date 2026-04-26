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

	// Precedence (v0.3-M6): betrayal/war_started > battle_resolved > trade_dispute/market_boom > alliance/treaty (diplomacy+) > trade_agreement.
	// This preserves the "at most 1 extra story event per intent" constraint.
	if strings.Contains(g, "背叛") || strings.Contains(g, "翻脸") {
		return storyEventSpec{
			typ:       "betrayal",
			narrative: "关系裂变：背叛发生（目标：" + g + "）",
			delta:     map[string]any{"trust": int64(-10), "conflict": int64(8), "order": int64(-2)},
		}, true
	}
	if strings.Contains(g, "宣战") || strings.Contains(g, "开战") {
		return storyEventSpec{
			typ:       "war_started",
			narrative: "战端开启：宣战（目标：" + g + "）",
			delta:     map[string]any{"conflict": int64(10), "order": int64(-3), "trust": int64(-4)},
		}, true
	}
	if strings.Contains(g, "进攻") || strings.Contains(g, "突袭") || strings.Contains(g, "战斗") || strings.Contains(g, "会战") {
		return storyEventSpec{
			typ:       "battle_resolved",
			narrative: "战斗结算：一场会战尘埃落定（目标：" + g + "）",
			delta:     map[string]any{"conflict": int64(6), "order": int64(-2), "trust": int64(-2), "food": int64(-2)},
		}, true
	}

	// Precedence (v0.3-M4): trade deepen branch (dispute/boom) beats diplomacy+ and base trade.
	if strings.Contains(g, "封锁") || strings.Contains(g, "禁运") || strings.Contains(g, "加税") || strings.Contains(g, "关税") {
		return storyEventSpec{
			typ:       "trade_dispute",
			narrative: "贸易纠纷：封锁与反制（目标：" + g + "）",
			delta:     map[string]any{"food": int64(-3), "trust": int64(-6), "conflict": int64(4), "order": int64(-1)},
		}, true
	}
	if strings.Contains(g, "繁荣") || strings.Contains(g, "互市") || strings.Contains(g, "市场") || strings.Contains(g, "开放贸易") {
		return storyEventSpec{
			typ:       "market_boom",
			narrative: "贸易繁荣：市场兴旺（目标：" + g + "）",
			delta:     map[string]any{"food": int64(8), "knowledge": int64(2), "trust": int64(2), "conflict": int64(-1)},
		}, true
	}

	// Precedence: diplomacy+ beats base trade_agreement.
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
