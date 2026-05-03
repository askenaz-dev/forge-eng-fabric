package githubapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Cache is the small Redis-backed cache surface needed by the GitHub service.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// Repository is the subset of GitHub repository metadata surfaced by Phase 0.
type Repository struct {
	ID            int64  `json:"id,omitempty"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

// Installation identifies a GitHub App installation recorded for a workspace.
type Installation struct {
	InstallationID string
	GitHubAccount  string
}

type Config struct {
	RedisURL          string
	GitHubAPIURL      string
	InstallationToken string
	FixtureJSON       string
	CacheTTL          time.Duration
}

// Service lists repositories for a recorded GitHub App installation.
type Service struct {
	cache    Cache
	http     *http.Client
	apiURL   string
	token    string
	fixture  []Repository
	cacheTTL time.Duration
}

func NewService(cfg Config) (*Service, error) {
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	apiURL := strings.TrimRight(cfg.GitHubAPIURL, "/")
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}

	var fixture []Repository
	if strings.TrimSpace(cfg.FixtureJSON) != "" {
		if err := json.Unmarshal([]byte(cfg.FixtureJSON), &fixture); err != nil {
			return nil, fmt.Errorf("parse github repositories fixture: %w", err)
		}
	}

	var cache Cache
	if strings.TrimSpace(cfg.RedisURL) != "" {
		redisCache, err := NewRedisCache(cfg.RedisURL)
		if err != nil {
			return nil, err
		}
		cache = redisCache
	}

	return &Service{
		cache:    cache,
		http:     &http.Client{Timeout: 10 * time.Second},
		apiURL:   apiURL,
		token:    strings.TrimSpace(cfg.InstallationToken),
		fixture:  fixture,
		cacheTTL: cfg.CacheTTL,
	}, nil
}

func (s *Service) ListRepositories(ctx context.Context, installation Installation, refresh bool) ([]Repository, bool, error) {
	if installation.InstallationID == "" {
		return nil, false, errors.New("installation_id is required")
	}

	cacheKey := "forge:github:repos:installation:" + installation.InstallationID
	if s.cache != nil && !refresh {
		if cached, ok, err := s.cache.Get(ctx, cacheKey); err == nil && ok {
			var repos []Repository
			if err := json.Unmarshal(cached, &repos); err == nil {
				return repos, true, nil
			}
		}
	}

	repos, err := s.fetchRepositories(ctx, installation)
	if err != nil {
		return nil, false, err
	}
	if s.cache != nil {
		if encoded, err := json.Marshal(repos); err == nil {
			_ = s.cache.Set(ctx, cacheKey, encoded, s.cacheTTL)
		}
	}
	return repos, false, nil
}

func (s *Service) fetchRepositories(ctx context.Context, installation Installation) ([]Repository, error) {
	if s.token == "" {
		return s.fixtureRepositories(installation), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL+"/installation/repositories?per_page=100", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/vnd.github+json")
	req.Header.Set("authorization", "Bearer "+s.token)
	req.Header.Set("x-github-api-version", "2022-11-28")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("github repositories %d: %s", resp.StatusCode, string(body))
	}

	var out struct {
		Repositories []Repository `json:"repositories"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Repositories, nil
}

func (s *Service) fixtureRepositories(installation Installation) []Repository {
	if len(s.fixture) > 0 {
		return append([]Repository(nil), s.fixture...)
	}
	account := strings.TrimSpace(installation.GitHubAccount)
	if account == "" {
		account = "forge-local"
	}
	return []Repository{
		{
			ID:            1,
			Name:          "forge-local",
			FullName:      account + "/forge-local",
			Private:       true,
			HTMLURL:       "https://github.com/" + account + "/forge-local",
			DefaultBranch: "main",
		},
	}
}
