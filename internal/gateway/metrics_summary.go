package gateway

import (
	"fmt"
	"math"
	"sort"

	"lobster-world-core/internal/sim"
)

// buildMetricsSummary produces a low-cardinality, human-readable snapshot for debug/metrics.
//
// It is intentionally:
//   - O(worlds) (acceptable for debug endpoints)
//   - string-based (not used for machine alerting)
//   - derived from existing structured fields (busy_by_reason, world_queue_stats, world_tick_stats)
func buildMetricsSummary(mt *Metrics, qs map[string]sim.QueueStat, ts map[string]sim.TickStat) map[string]any {
	out := map[string]any{
		"busy":  "unknown",
		"queue": "unknown",
		"tick":  "unknown",
	}

	// --- busy ---
	if mt == nil {
		out["busy"] = "no_metrics"
	} else {
		snap := mt.Snapshot()
		busyTotal, _ := snap["busy_total"].(int64)
		if busyTotal == 0 {
			out["busy"] = "ok"
		} else {
			by, _ := snap["busy_by_reason"].(map[string]any)
			out["busy"] = fmt.Sprintf(
				"busy_total=%d (intent_ch_full=%d, pending_queue_full=%d, accept_timeout=%d)",
				busyTotal,
				asInt64(by, "intent_ch_full"),
				asInt64(by, "pending_queue_full"),
				asInt64(by, "accept_timeout"),
			)
		}
	}

	// --- queue ---
	if len(qs) == 0 {
		out["queue"] = "no_worlds"
	} else {
		keys := make([]string, 0, len(qs))
		for k := range qs {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		maxPending := 0
		maxIntentCh := 0

		hottest := ""
		hottestScore := -1.0
		hottestPendingLine := ""

		for _, wid := range keys {
			q := qs[wid]
			if q.PendingQueueMax > maxPending {
				maxPending = q.PendingQueueMax
			}
			if q.IntentChCap > maxIntentCh {
				maxIntentCh = q.IntentChCap
			}

			pr := ratio(q.PendingQueueLen, q.PendingQueueMax)
			ir := ratio(q.IntentChLen, q.IntentChCap)
			score := math.Max(pr, ir)

			// Stable selection: score first, then world_id.
			if score > hottestScore || (score == hottestScore && (hottest == "" || wid < hottest)) {
				hottestScore = score
				hottest = wid
				hottestPendingLine = fmt.Sprintf(
					"pending=%d/%d intent_ch=%d/%d tick=%d",
					q.PendingQueueLen, q.PendingQueueMax,
					q.IntentChLen, q.IntentChCap,
					q.Tick,
				)
			}
		}

		out["queue"] = fmt.Sprintf(
			"worlds=%d max_pending=%d max_intent_ch=%d hottest=%s %s",
			len(qs), maxPending, maxIntentCh, hottest, hottestPendingLine,
		)
	}

	// --- tick ---
	if len(ts) == 0 {
		out["tick"] = "no_worlds"
	} else {
		overrunTotal := int64(0)
		jitterMsTotal := int64(0)
		jitterCount := int64(0)

		hottest := ""
		hottestOverrun := int64(-1)

		keys := make([]string, 0, len(ts))
		for k := range ts {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, wid := range keys {
			st := ts[wid]
			overrunTotal += st.TickOverrunTotal
			jitterMsTotal += st.TickJitterMsTotal
			jitterCount += st.TickJitterCount

			if st.TickOverrunTotal > hottestOverrun || (st.TickOverrunTotal == hottestOverrun && (hottest == "" || wid < hottest)) {
				hottestOverrun = st.TickOverrunTotal
				hottest = wid
			}
		}

		avg := 0.0
		if jitterCount > 0 {
			avg = float64(jitterMsTotal) / float64(jitterCount)
		}

		out["tick"] = fmt.Sprintf(
			"worlds=%d overrun_total=%d jitter_avg_ms≈%.1f hottest=%s overrun=%d",
			len(ts), overrunTotal, avg, hottest, hottestOverrun,
		)
	}

	return out
}

func ratio(a, b int) float64 {
	if b <= 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func asInt64(m map[string]any, k string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		// Defensive: in case callers pass JSON-decoded maps.
		return int64(t)
	default:
		return 0
	}
}

