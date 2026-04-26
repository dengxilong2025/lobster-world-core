package gateway

import (
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// startTime is captured once per process, for /api/v0/debug/build uptime reporting.
var startTime = time.Now()

func buildInfoSnapshot() map[string]any {
	out := map[string]any{
		"start_time": startTime.UTC().Format(time.RFC3339),
		"uptime_sec": int64(time.Since(startTime).Seconds()),
		// Fallback in case runtime/debug.ReadBuildInfo is unavailable.
		"go_version": runtime.Version(),
	}

	// Go build metadata (best-effort).
	if bi, ok := debug.ReadBuildInfo(); ok && bi != nil {
		if bi.GoVersion != "" {
			out["go_version"] = bi.GoVersion
		}
		if bi.Main.Path != "" {
			// Prefer last path segment for readability.
			parts := strings.Split(bi.Main.Path, "/")
			out["module"] = parts[len(parts)-1]
			// Keep stable shape: module_version exists even if empty.
			out["module_version"] = bi.Main.Version
		}

		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if s.Value != "" {
					sha := s.Value
					if len(sha) > 7 {
						sha = sha[:7]
					}
					out["git_sha"] = sha
				}
			case "vcs.time":
				if s.Value != "" {
					out["vcs_time"] = s.Value
				}
			case "vcs.modified":
				if s.Value != "" {
					out["vcs_modified"] = (s.Value == "true")
				}
			}
		}
	}

	return out
}

