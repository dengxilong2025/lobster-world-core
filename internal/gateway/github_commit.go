package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultGitHubAPIBaseURL = "https://api.github.com"
	defaultGitHubCacheTTL   = 5 * time.Minute
	defaultGitHubTimeout    = 2 * time.Second
)

// GitHubCommitResolver resolves a repo+branch to the latest short SHA via GitHub API.
// Used only as a debug/build fallback when git_sha is "unknown".
type GitHubCommitResolver interface {
	LatestSHA7(repoSlug, branch string) (string, error)
}

type GitHubCommitResolverOptions struct {
	// BaseURL is mainly for tests (e.g. httptest.Server).
	// Default: https://api.github.com
	BaseURL string

	// Client is the HTTP client used for requests.
	// If nil, a new client with a 2s timeout is used.
	// If non-nil and Timeout==0, a shallow copy is made and Timeout is set to 2s.
	Client *http.Client

	// TTL controls the in-process cache TTL.
	// Default: 5 minutes.
	TTL time.Duration

	// Now is injected for deterministic cache tests.
	// Default: time.Now
	Now func() time.Time
}

type githubCommitResolver struct {
	mu    sync.Mutex
	cache map[string]cachedSHA7

	ttl    time.Duration
	client *http.Client
	base   string
	now    func() time.Time
}

type cachedSHA7 struct {
	sha7 string
	exp  time.Time
}

func NewGitHubCommitResolver(opts GitHubCommitResolverOptions) GitHubCommitResolver {
	base := strings.TrimSpace(opts.BaseURL)
	if base == "" {
		base = defaultGitHubAPIBaseURL
	}

	ttl := opts.TTL
	if ttl <= 0 {
		ttl = defaultGitHubCacheTTL
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}

	var c *http.Client
	switch {
	case opts.Client == nil:
		c = &http.Client{Timeout: defaultGitHubTimeout}
	case opts.Client.Timeout == 0:
		cp := *opts.Client
		cp.Timeout = defaultGitHubTimeout
		c = &cp
	default:
		c = opts.Client
	}

	return &githubCommitResolver{
		cache:  map[string]cachedSHA7{},
		ttl:    ttl,
		client: c,
		base:   strings.TrimRight(base, "/"),
		now:    now,
	}
}

func (r *githubCommitResolver) LatestSHA7(repoSlug, branch string) (string, error) {
	repoSlug = strings.TrimSpace(repoSlug)
	branch = strings.TrimSpace(branch)
	if repoSlug == "" || branch == "" {
		return "", fmt.Errorf("github commit resolver: empty repoSlug or branch")
	}
	if strings.Contains(repoSlug, " ") || strings.Contains(branch, " ") {
		return "", fmt.Errorf("github commit resolver: invalid repoSlug or branch")
	}

	key := repoSlug + ":" + branch
	now := r.now()

	r.mu.Lock()
	if v, ok := r.cache[key]; ok && v.sha7 != "" && now.Before(v.exp) {
		r.mu.Unlock()
		return v.sha7, nil
	}
	r.mu.Unlock()

	u := fmt.Sprintf("%s/repos/%s/commits/%s", r.base, repoSlug, url.PathEscape(branch))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lobster-world-core")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api status=%d", resp.StatusCode)
	}

	var body struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}

	sha := strings.TrimSpace(body.SHA)
	if len(sha) < 7 {
		return "", fmt.Errorf("github api: invalid sha")
	}
	sha7 := sha[:7]

	r.mu.Lock()
	r.cache[key] = cachedSHA7{sha7: sha7, exp: now.Add(r.ttl)}
	r.mu.Unlock()

	return sha7, nil
}

