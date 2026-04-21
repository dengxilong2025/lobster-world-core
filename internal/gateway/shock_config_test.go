package gateway

import "testing"

func TestShockConfigFromEnv_DisabledReturnsNil(t *testing.T) {
	t.Setenv("LW_SHOCK_ENABLED", "0")
	if got := ShockConfigFromEnv(); got != nil {
		t.Fatalf("expected nil when disabled, got %#v", got)
	}
}

func TestShockConfigFromEnv_ParsesNumericOverrides(t *testing.T) {
	t.Setenv("LW_SHOCK_ENABLED", "1")
	t.Setenv("LW_SHOCK_EPOCH_TICKS", "10")
	t.Setenv("LW_SHOCK_WARNING_OFFSET", "2")
	t.Setenv("LW_SHOCK_DURATION_TICKS", "3")
	t.Setenv("LW_SHOCK_COOLDOWN_TICKS", "11")

	cfg := ShockConfigFromEnv()
	if cfg == nil {
		t.Fatalf("expected config")
	}
	if cfg.EpochTicks != 10 || cfg.WarningOffset != 2 || cfg.DurationTicks != 3 || cfg.CooldownTicks != 11 {
		t.Fatalf("unexpected cfg: %#v", cfg)
	}
	if len(cfg.Candidates) < 3 {
		t.Fatalf("expected >=3 built-in candidates, got %d", len(cfg.Candidates))
	}
	seen := map[string]bool{}
	for _, c := range cfg.Candidates {
		if c.Key == "" {
			t.Fatalf("unexpected empty key: %#v", c)
		}
		if seen[c.Key] {
			t.Fatalf("duplicate key %q", c.Key)
		}
		seen[c.Key] = true
	}
}
