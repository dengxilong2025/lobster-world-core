package gateway

import (
	"net/http"
	"time"

	"lobster-world-core/internal/sim"
)

func registerDebugRoutes(mux *http.ServeMux, sm *sim.Engine, trustedProxyCIDRs []string, mt *Metrics) {
	mux.HandleFunc("GET /api/v0/debug/config", func(w http.ResponseWriter, r *http.Request) {
		cfg := map[string]any{
			"trusted_proxy_cidrs": trustedProxyCIDRs,
		}

		if sm != nil {
			ec := sm.Config()
			cfg["tick_interval_ms"] = int64(ec.TickInterval / time.Millisecond)
			cfg["intent_accept_timeout_ms"] = int64(ec.IntentAcceptTimeout / time.Millisecond)
			cfg["max_intent_queue"] = ec.MaxIntentQueue

			if ec.Shock != nil {
				cfg["shock_enabled"] = true
				cfg["shock_epoch_ticks"] = ec.Shock.EpochTicks
				cfg["shock_warning_offset"] = ec.Shock.WarningOffset
				cfg["shock_duration_ticks"] = ec.Shock.DurationTicks
				cfg["shock_cooldown_ticks"] = ec.Shock.CooldownTicks
				keys := make([]string, 0, len(ec.Shock.Candidates))
				for _, c := range ec.Shock.Candidates {
					if c.Key != "" {
						keys = append(keys, c.Key)
					}
				}
				cfg["shock_candidate_keys"] = keys
			} else {
				cfg["shock_enabled"] = false
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"config": cfg,
		})
	})

	mux.HandleFunc("GET /api/v0/debug/metrics", func(w http.ResponseWriter, r *http.Request) {
		snap := map[string]any{}
		if mt != nil {
			snap = mt.Snapshot()
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"metrics": snap,
		})
	})
}
