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
				ActorsPool:        []string{"nation_a", "nation_b", "nation_c"},
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

