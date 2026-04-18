package gateway

import (
	"strings"

	"lobster-world-core/internal/projections/spectator"
)

// pickRecentNarratives extracts up to N distinct narratives from spectator.Home,
// preferring headline first, then hot events in order.
func pickRecentNarratives(home spectator.Home, n int) []string {
	if n <= 0 {
		n = 2
	}
	out := make([]string, 0, n)
	seen := map[string]struct{}{}

	add := func(s string) {
		if len(out) >= n {
			return
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	if home.Headline != nil {
		add(home.Headline.Narrative)
	}
	for _, e := range home.HotEvents {
		add(e.Narrative)
		if len(out) >= n {
			break
		}
	}
	return out
}

