package gateway

import (
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// startTime is captured once per process, for /api/v0/debug/build uptime reporting.
var startTime = time.Now()

// buildGitSHA is injected at build time (Dockerfile ldflags). It should be a short SHA (7 chars).
// If empty, buildInfoSnapshot falls back to buildvcs (vcs.revision) or "unknown".
var buildGitSHA string

const (
	defaultRenderRepoSlug = "dengxilong2025/lobster-world-core"
	defaultRenderBranch   = "main"
)

func buildInfoSnapshot(readBuildInfo func() (*debug.BuildInfo, bool), gh GitHubCommitResolver) map[string]any {
	out := map[string]any{
		"start_time": startTime.UTC().Format(time.RFC3339),
		"uptime_sec": int64(time.Since(startTime).Seconds()),
		// Fallback in case runtime/debug.ReadBuildInfo is unavailable.
		"go_version": runtime.Version(),
	}

	// Go build metadata (best-effort).
	if readBuildInfo == nil {
		readBuildInfo = debug.ReadBuildInfo
	}
	if bi, ok := readBuildInfo(); ok && bi != nil {
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

	// Strong guarantee: git_sha is always present and non-empty.
	// Order:
	// 1) buildvcs: vcs.revision (above)
	// 2) build-time injected ldflags (buildGitSHA)
	// 3) Render env: RENDER_GIT_COMMIT
	// 4) GitHub API fallback (only when git_sha=="unknown")
	if gitSHA, _ := out["git_sha"].(string); strings.TrimSpace(gitSHA) == "" {
		if buildGitSHA != "" {
			out["git_sha"] = buildGitSHA
		} else if env := strings.TrimSpace(os.Getenv("RENDER_GIT_COMMIT")); env != "" {
			if len(env) > 7 {
				env = env[:7]
			}
			out["git_sha"] = env
		} else {
			out["git_sha"] = "unknown"
		}
	}

	// GitHub public API fallback: only when current git_sha is "unknown" (or missing).
	if gitSHA, _ := out["git_sha"].(string); strings.TrimSpace(gitSHA) == "" || gitSHA == "unknown" {
		if gh != nil {
			repoSlug := strings.TrimSpace(os.Getenv("RENDER_GIT_REPO_SLUG"))
			branch := strings.TrimSpace(os.Getenv("RENDER_GIT_BRANCH"))
			if repoSlug == "" || branch == "" {
				repoSlug = defaultRenderRepoSlug
				branch = defaultRenderBranch
			}
			if sha7, err := gh.LatestSHA7(repoSlug, branch); err == nil && sha7 != "" {
				out["git_sha"] = sha7
			}
		}
	}

	// Final guarantee.
	if gitSHA, _ := out["git_sha"].(string); strings.TrimSpace(gitSHA) == "" {
		out["git_sha"] = "unknown"
	}

	return out
}
