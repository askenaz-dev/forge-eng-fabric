package ghpackages_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	adapter "github.com/forge-eng-fabric/pkg/artifact-store-adapter"
	_ "github.com/forge-eng-fabric/pkg/artifact-store-adapter/ghpackages"
)

// githubFake serves the small subset of the GitHub REST API the driver
// uses: /repos/{owner}/{repo}, /repos/.../releases, /repos/.../releases/tags/{tag},
// /repos/.../releases/{id}/assets (upload via uploads.github.com surrogate).
type githubFake struct {
	mu          sync.Mutex
	private     bool
	releases    map[string]*release
	assetsByID  map[int64]*assetBlob
	nextID      int64
	healthFails bool
	// selfURL is set after the test server is created; the fake uses it to
	// emit asset URLs that loop back to itself.
	selfURL string
}

type release struct {
	ID         int64    `json:"id"`
	TagName    string   `json:"tag_name"`
	UploadURL  string   `json:"upload_url"`
	Assets     []asset  `json:"assets"`
	tagPath    string   // internal tag key (matches /releases/tags/<tag>)
}

type asset struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	BrowserURL string `json:"browser_download_url"`
}

type assetBlob struct {
	name string
	data []byte
}

func newGithubFake(private bool) *githubFake {
	return &githubFake{
		private: private, releases: map[string]*release{}, assetsByID: map[int64]*assetBlob{},
	}
}

func (g *githubFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// GET /repos/{owner}/{repo}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/repos/") && !strings.Contains(r.URL.Path, "/releases") && !strings.Contains(r.URL.Path, "/assets") {
		if g.healthFails {
			w.WriteHeader(503)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"private": g.private})
		return
	}

	// GET /repos/.../releases/tags/{tag}
	if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/") {
		tag := strings.TrimPrefix(r.URL.Path, "")
		idx := strings.Index(tag, "/releases/tags/")
		if idx >= 0 {
			tag = tag[idx+len("/releases/tags/"):]
		}
		rel, ok := g.releases[tag]
		if !ok {
			w.WriteHeader(404)
			return
		}
		_ = json.NewEncoder(w).Encode(rel)
		return
	}

	// POST /repos/.../releases
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/releases") {
		var body struct {
			TagName string `json:"tag_name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		g.nextID++
		rel := &release{
			ID:        g.nextID,
			TagName:   body.TagName,
			tagPath:   body.TagName,
		}
		g.releases[body.TagName] = rel
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(rel)
		return
	}

	// POST /repos/.../releases/{id}/assets   (via uploads.github.com)
	if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/releases/") && strings.HasSuffix(r.URL.Path, "/assets") {
		var releaseID int64
		// Extract the id from the path.
		parts := strings.Split(r.URL.Path, "/")
		for i, p := range parts {
			if p == "releases" && i+1 < len(parts) {
				_, _ = fmt.Sscanf(parts[i+1], "%d", &releaseID)
				break
			}
		}
		name := r.URL.Query().Get("name")
		var rel *release
		for _, v := range g.releases {
			if v.ID == releaseID {
				rel = v
				break
			}
		}
		if rel == nil {
			w.WriteHeader(404)
			return
		}
		for _, a := range rel.Assets {
			if a.Name == name {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
		}
		body, _ := io.ReadAll(r.Body)
		g.nextID++
		assetID := g.nextID
		g.assetsByID[assetID] = &assetBlob{name: name, data: body}
		rel.Assets = append(rel.Assets, asset{ID: assetID, Name: name, URL: fmt.Sprintf("%s/repos/x/x/releases/assets/%d", g.selfURL, assetID)})
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(rel.Assets[len(rel.Assets)-1])
		return
	}

	// GET /repos/.../releases/assets/{id}
	if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/assets/") {
		var id int64
		_, _ = fmt.Sscanf(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], "%d", &id)
		blob, ok := g.assetsByID[id]
		if !ok {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write(blob.data)
		return
	}

	// DELETE /repos/.../releases/{id}
	if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/releases/") && !strings.Contains(r.URL.Path, "/assets") {
		w.WriteHeader(204)
		return
	}

	w.WriteHeader(404)
}

func newDriver(t *testing.T, apiURL, uploadURL string, tenant string, private bool) (adapter.Adapter, *githubFake) {
	t.Helper()
	f := adapter.NewFactory(adapter.StaticSecretFetcher{"vault://forge/ghpat": []byte("ghp_xxx")}, &auditNoOp{})
	a, err := f.Build(context.Background(), adapter.BindingConfig{
		TenantID:      tenant,
		Backend:       adapter.BackendGitHubPackages,
		CredentialRef: "vault://forge/ghpat",
		Settings: map[string]any{
			"owner":           "forge",
			"repo_prefix":     "forge-skills",
			"api_base_url":    apiURL,
			"upload_base_url": uploadURL,
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return a, nil
}

type auditNoOp struct{}

func (auditNoOp) EmitArtifactEvent(context.Context, adapter.AuditEvent) error { return nil }

func TestGithubPackagesContract(t *testing.T) {
	fake := newGithubFake(true)
	srv := httptest.NewServer(fake)
	defer srv.Close()
	fake.selfURL = srv.URL
	a, _ := newDriver(t, srv.URL, srv.URL, "t1", true)

	payload := []byte("github private skill bytes")
	digest := adapter.DigestSHA256(payload)
	obj := adapter.Object{TenantID: "t1", AssetID: "skill-gh", Version: "1.0.0", Digest: digest}
	if err := a.Put(context.Background(), obj, strings.NewReader(string(payload)), int64(len(payload)), adapter.Manifest{Digest: digest}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	rc, err := a.Get(context.Background(), obj)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(rc)
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("bytes mismatch")
	}

	// Second Put for the same asset name fails 422 → ErrCodeImmutable.
	if err := a.Put(context.Background(), obj, strings.NewReader(string(payload)), int64(len(payload)), adapter.Manifest{}); !adapter.IsCode(err, adapter.ErrCodeImmutable) {
		t.Fatalf("expected ErrCodeImmutable; got %v", err)
	}

	other := obj
	other.TenantID = "t2"
	if _, err := a.Get(context.Background(), other); !adapter.IsCode(err, adapter.ErrCodeCrossTenant) {
		t.Fatalf("expected ErrCodeCrossTenant; got %v", err)
	}
}

func TestGithubPackagesRejectsPublicRepo(t *testing.T) {
	fake := newGithubFake(false) // private=false → public
	srv := httptest.NewServer(fake)
	defer srv.Close()

	f := adapter.NewFactory(adapter.StaticSecretFetcher{"vault://forge/ghpat": []byte("ghp_xxx")}, &auditNoOp{})
	_, err := f.Build(context.Background(), adapter.BindingConfig{
		TenantID:      "t1",
		Backend:       adapter.BackendGitHubPackages,
		CredentialRef: "vault://forge/ghpat",
		Settings: map[string]any{
			"owner":           "forge",
			"repo_prefix":     "forge-skills",
			"api_base_url":    srv.URL,
			"upload_base_url": srv.URL,
		},
	})
	if !adapter.IsCode(err, adapter.ErrCodePublicBackend) {
		t.Fatalf("expected ErrCodePublicBackend for public repo; got %v", err)
	}
}
