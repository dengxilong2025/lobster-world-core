package sim

import "time"

// EngineConfig is a safe snapshot of engine runtime configuration (no secrets).
type EngineConfig struct {
	TickInterval        time.Duration `json:"-"`
	IntentAcceptTimeout time.Duration `json:"-"`
	MaxIntentQueue      int
	IntentChannelCap    int
	Shock               *ShockConfig // nil when disabled
}

func (e *Engine) Config() EngineConfig {
	e.mu.Lock()
	defer e.mu.Unlock()

	var shockCopy *ShockConfig
	if e.shock != nil {
		tmp := *e.shock
		tmp.Candidates = append([]ShockCandidate{}, e.shock.Candidates...)
		shockCopy = &tmp
	}

	return EngineConfig{
		TickInterval:        e.tickInterval,
		IntentAcceptTimeout: e.intentAcceptTimeout,
		MaxIntentQueue:      e.maxIntentQueue,
		IntentChannelCap:    e.intentChannelCap,
		Shock:               shockCopy,
	}
}
