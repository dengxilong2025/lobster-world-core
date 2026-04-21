package gateway

import (
	"os"
	"strconv"
	"strings"

	"lobster-world-core/internal/sim"
)

// ShockConfigFromEnv builds a ShockConfig for production tuning.
//
// Env vars (v0):
// - LW_SHOCK_ENABLED=1 enables the scheduler
// - LW_SHOCK_EPOCH_TICKS (default: 720)
// - LW_SHOCK_WARNING_OFFSET (default: 1)
// - LW_SHOCK_DURATION_TICKS (default: 3)
// - LW_SHOCK_COOLDOWN_TICKS (default: 720)
//
// This intentionally keeps candidate configuration simple in v0:
// we use a small built-in candidate set, while allowing numeric tuning per env.
func ShockConfigFromEnv() *sim.ShockConfig {
	if strings.TrimSpace(os.Getenv("LW_SHOCK_ENABLED")) != "1" {
		return nil
	}

	cfg := sim.ShockConfig{
		EpochTicks:    720,
		WarningOffset: 1,
		DurationTicks: 3,
		CooldownTicks: 720,
		Candidates: []sim.ShockCandidate{
			{
				Key:              "riftwinter",
				Weight:           1,
				WarningNarrative:  "天象异常：裂冬指数上升",
				StartedNarrative:  "冲击开始：裂冬纪元降临",
				EndedNarrative:    "冲击结束：裂冬余波仍在",
				WarningDelta:     map[string]int64{"food": -3, "order": -1},
				StartedDelta:     map[string]int64{"food": -8, "order": -4, "trust": -2, "conflict": +2},
				EndedDelta:       map[string]int64{"knowledge": +3},
				ActorsPool:        []string{"nation_a", "nation_b", "nation_c"},
			},
			{
				Key:              "greatdrought",
				Weight:           1,
				WarningNarrative: "预警：河道见底，旱季异常延长",
				StartedNarrative: "冲击开始：大旱席卷，粮价暴涨",
				EndedNarrative:   "冲击结束：旱季缓解，但创伤仍在",
				WarningDelta:     map[string]int64{"food": -4},
				StartedDelta:     map[string]int64{"food": -12, "order": -3, "conflict": +3},
				EndedDelta:       map[string]int64{"order": +1},
				ActorsPool:       []string{"nation_a", "nation_d", "nation_e"},
			},
			{
				Key:              "plaguewave",
				Weight:           1,
				WarningNarrative: "预警：疫病风声四起，集市开始封闭",
				StartedNarrative: "冲击开始：疫病浪潮爆发，城市停摆",
				EndedNarrative:   "冲击结束：疫病退潮，社会结构重组",
				WarningDelta:     map[string]int64{"order": -2, "trust": -1},
				StartedDelta:     map[string]int64{"population": -6, "order": -4, "trust": -3},
				EndedDelta:       map[string]int64{"knowledge": +4, "trust": +1},
				ActorsPool:       []string{"nation_b", "nation_f", "nation_g"},
			},
			{
				Key:              "outsiderraid",
				Weight:           1,
				WarningNarrative: "预警：边境烟尘，外敌动向可疑",
				StartedNarrative: "冲击开始：外族入侵，边境告急",
				EndedNarrative:   "冲击结束：入侵暂退，仇怨被点燃",
				WarningDelta:     map[string]int64{"conflict": +2},
				StartedDelta:     map[string]int64{"conflict": +8, "order": -3, "trust": -2},
				EndedDelta:       map[string]int64{"conflict": -2},
				ActorsPool:       []string{"nation_c", "nation_h", "nation_i"},
			},
		},
	}

	if v := parseEnvInt64("LW_SHOCK_EPOCH_TICKS"); v > 0 {
		cfg.EpochTicks = v
	}
	if v := parseEnvInt64("LW_SHOCK_WARNING_OFFSET"); v >= 0 {
		cfg.WarningOffset = v
	}
	if v := parseEnvInt64("LW_SHOCK_DURATION_TICKS"); v > 0 {
		cfg.DurationTicks = v
	}
	if v := parseEnvInt64("LW_SHOCK_COOLDOWN_TICKS"); v >= 0 {
		cfg.CooldownTicks = v
	}

	return &cfg
}

func parseEnvInt64(key string) int64 {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
