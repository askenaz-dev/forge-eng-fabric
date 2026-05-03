package githubapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type memoryCache struct {
	values map[string][]byte
}

func (m *memoryCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	value, ok := m.values[key]
	return value, ok, nil
}

func (m *memoryCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.values[key] = append([]byte(nil), value...)
	return nil
}

func TestListRepositoriesUsesFixtureAndCache(t *testing.T) {
	svc, err := NewService(Config{
		FixtureJSON: `[{"name":"demo","full_name":"acme/demo","private":true,"default_branch":"main"}]`,
		CacheTTL:    time.Minute,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.cache = &memoryCache{values: map[string][]byte{}}

	repos, cacheHit, err := svc.ListRepositories(context.Background(), Installation{InstallationID: "local", GitHubAccount: "acme"}, false)
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if cacheHit {
		t.Fatal("first call should not hit cache")
	}
	if len(repos) != 1 || repos[0].FullName != "acme/demo" {
		t.Fatalf("unexpected repositories: %#v", repos)
	}

	repos, cacheHit, err = svc.ListRepositories(context.Background(), Installation{InstallationID: "local", GitHubAccount: "acme"}, false)
	if err != nil {
		t.Fatalf("ListRepositories cached: %v", err)
	}
	if !cacheHit {
		t.Fatal("second call should hit cache")
	}
	if len(repos) != 1 || repos[0].FullName != "acme/demo" {
		t.Fatalf("unexpected cached repositories: %#v", repos)
	}
}

func TestListRepositoriesCallsGitHubWhenTokenConfigured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/installation/repositories" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("authorization"); got != "Bearer token" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"repositories":[{"id":42,"name":"demo","full_name":"acme/demo","private":false}]}`))
	}))
	defer server.Close()

	svc, err := NewService(Config{GitHubAPIURL: server.URL, InstallationToken: "token"})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	repos, cacheHit, err := svc.ListRepositories(context.Background(), Installation{InstallationID: "real", GitHubAccount: "acme"}, false)
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}
	if cacheHit {
		t.Fatal("call without cache should not report a cache hit")
	}
	if len(repos) != 1 || repos[0].ID != 42 || repos[0].FullName != "acme/demo" {
		t.Fatalf("unexpected repositories: %#v", repos)
	}
}
