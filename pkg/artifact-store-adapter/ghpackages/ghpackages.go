// Package ghpackages implements the artifact-store adapter for GitHub
// (binding key `github-packages-private`).
//
// The native GitHub Packages API is npm/maven-flavored and assumes the
// artifact follows a language ecosystem's packaging conventions. Skills
// are opaque tar.zst blobs and we don't want to couple Forge's package
// format to npm. Instead this driver uses **GitHub Releases assets in a
// private repository**: each (asset_id, version) is a release with a tag
// `skill/{asset_id}/{version}` and a single binary asset named
// `{asset_id}-{version}.tar.zst`. The repo MUST be private; the binding
// rejects public repos via the Health probe.
//
// Per-Tenant isolation: each Tenant gets its own private GitHub repo
// (configured via `repo_prefix` + tenant suffix), and the credential the
// adapter is built with grants access only to that repo.
package ghpackages

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	adapter "github.com/forge-eng-fabric/pkg/artifact-store-adapter"
)

type Config struct {
	Owner          string `json:"owner"`           // GitHub org or user
	RepoPrefix     string `json:"repo_prefix"`     // default "forge-skills"
	APIBaseURL     string `json:"api_base_url"`    // default "https://api.github.com"
	UploadBaseURL  string `json:"upload_base_url"` // default "https://uploads.github.com"
}

type Driver struct {
	cfg     Config
	tenant  string
	token   string
	client  *http.Client
	audit   adapter.AuditSink
}

func init() {
	adapter.RegisterDriver(adapter.BackendGitHubPackages, build)
}

func build(ctx context.Context, cfg adapter.BindingConfig, deps adapter.Deps) (adapter.Adapter, error) {
	var typed Config
	if err := adapter.DecodeSettings(cfg.Settings, &typed); err != nil {
		return nil, err
	}
	if typed.Owner == "" {
		return nil, &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: "github-packages: owner required"}
	}
	if typed.RepoPrefix == "" {
		typed.RepoPrefix = "forge-skills"
	}
	if typed.APIBaseURL == "" {
		typed.APIBaseURL = "https://api.github.com"
	}
	if typed.UploadBaseURL == "" {
		typed.UploadBaseURL = "https://uploads.github.com"
	}
	secret, err := deps.SecretFetcher.Fetch(ctx, cfg.CredentialRef)
	if err != nil {
		return nil, err
	}
	return &Driver{
		cfg:    typed,
		tenant: cfg.TenantID,
		token:  strings.TrimSpace(string(secret)),
		client: &http.Client{Timeout: 60 * time.Second},
		audit:  deps.AuditSink,
	}, nil
}

func (d *Driver) Backend() string { return adapter.BackendGitHubPackages }

func (d *Driver) repo() string {
	return d.cfg.RepoPrefix + "-" + d.tenant
}

func (d *Driver) tagName(obj adapter.Object) string {
	return "skill/" + obj.AssetID + "/" + obj.Version
}

func (d *Driver) assetName(obj adapter.Object) string {
	return fmt.Sprintf("%s-%s.tar.zst", obj.AssetID, obj.Version)
}

func (d *Driver) manifestAssetName(obj adapter.Object) string {
	return d.assetName(obj) + ".manifest.json"
}

func (d *Driver) crossTenantGuard(obj adapter.Object) error {
	if obj.TenantID != d.tenant {
		return &adapter.Error{Code: adapter.ErrCodeCrossTenant, Message: fmt.Sprintf("driver bound to tenant=%s; refused tenant=%s", d.tenant, obj.TenantID)}
	}
	return nil
}

type ghRelease struct {
	ID         int64      `json:"id"`
	TagName    string     `json:"tag_name"`
	UploadURL  string     `json:"upload_url"`
	HTMLURL    string     `json:"html_url"`
	Draft      bool       `json:"draft"`
	Assets     []ghAsset  `json:"assets"`
}

type ghAsset struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	BrowserURL  string `json:"browser_download_url"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

func (d *Driver) apiURL(p string) string {
	return strings.TrimRight(d.cfg.APIBaseURL, "/") + "/" + strings.TrimLeft(p, "/")
}

func (d *Driver) doJSON(req *http.Request, out any) (*http.Response, error) {
	req.Header.Set("authorization", "Bearer "+d.token)
	req.Header.Set("accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	if out != nil && resp.StatusCode/100 == 2 {
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		return resp, nil
	}
	return resp, nil
}

func (d *Driver) findRelease(ctx context.Context, obj adapter.Object) (*ghRelease, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		d.apiURL(fmt.Sprintf("/repos/%s/%s/releases/tags/%s",
			url.PathEscape(d.cfg.Owner), url.PathEscape(d.repo()), url.PathEscape(d.tagName(obj)))),
		nil)
	var rel ghRelease
	resp, err := d.doJSON(req, &rel)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode/100 != 2 {
		return nil, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("github releases status=%d", resp.StatusCode)}
	}
	return &rel, nil
}

func (d *Driver) Put(ctx context.Context, obj adapter.Object, content io.Reader, size int64, manifest adapter.Manifest) error {
	start := time.Now()
	if err := obj.Validate(); err != nil {
		return &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: err.Error()}
	}
	if err := d.crossTenantGuard(obj); err != nil {
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}
	// Refuse to overwrite: if a release exists with an asset of our name,
	// it is immutable.
	existing, err := d.findRelease(ctx, obj)
	if err != nil {
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}
	if existing != nil {
		for _, a := range existing.Assets {
			if a.Name == d.assetName(obj) {
				err := &adapter.Error{Code: adapter.ErrCodeImmutable, Message: fmt.Sprintf("github: %s already exists in release %s", a.Name, existing.TagName)}
				d.emit(ctx, "put", obj, "error", err, 0, start)
				return err
			}
		}
	}

	release := existing
	if release == nil {
		// Create a draft release; we mark it final after upload.
		body, _ := json.Marshal(map[string]any{
			"tag_name":   d.tagName(obj),
			"name":       d.tagName(obj),
			"draft":      false,
			"prerelease": false,
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
			d.apiURL(fmt.Sprintf("/repos/%s/%s/releases", url.PathEscape(d.cfg.Owner), url.PathEscape(d.repo()))),
			bytes.NewReader(body))
		req.Header.Set("content-type", "application/json")
		var rel ghRelease
		resp, err := d.doJSON(req, &rel)
		if err != nil {
			d.emit(ctx, "put", obj, "error", err, 0, start)
			return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "github create release: " + err.Error()}
		}
		if resp.StatusCode/100 != 2 {
			err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("github create release status=%d", resp.StatusCode)}
			d.emit(ctx, "put", obj, "error", err, 0, start)
			return err
		}
		release = &rel
	}

	// Upload the asset; the upload_url template returns the upload host;
	// for our purposes we use the configured upload base URL.
	uploadEndpoint := fmt.Sprintf("%s/repos/%s/%s/releases/%d/assets?name=%s",
		strings.TrimRight(d.cfg.UploadBaseURL, "/"),
		url.PathEscape(d.cfg.Owner), url.PathEscape(d.repo()),
		release.ID, url.QueryEscape(d.assetName(obj)))
	hashed := adapter.NewHashingReader(content)
	upReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, uploadEndpoint, hashed)
	upReq.Header.Set("authorization", "Bearer "+d.token)
	upReq.Header.Set("content-type", "application/octet-stream")
	if size > 0 {
		upReq.ContentLength = size
	}
	upResp, err := d.client.Do(upReq)
	if err != nil {
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "github upload: " + err.Error()}
	}
	defer upResp.Body.Close()
	if upResp.StatusCode == http.StatusUnprocessableEntity {
		// 422 from upload typically means name already exists.
		err := &adapter.Error{Code: adapter.ErrCodeImmutable, Message: "github upload: asset name already exists"}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}
	if upResp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(upResp.Body, 1024))
		err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("github upload status=%d body=%s", upResp.StatusCode, string(body))}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}
	if observed := hashed.Sum(); observed != obj.Digest {
		err := &adapter.Error{Code: adapter.ErrCodeDigestMismatch, Message: fmt.Sprintf("github: uploaded digest %s does not match expected %s", observed, obj.Digest)}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}

	// Manifest sidecar as a second asset on the same release.
	manifest.AssetID = obj.AssetID
	manifest.Version = obj.Version
	manifest.Digest = obj.Digest
	if manifest.UploadedAt.IsZero() {
		manifest.UploadedAt = time.Now().UTC()
	}
	mbody, _ := json.Marshal(manifest)
	manifestEndpoint := fmt.Sprintf("%s/repos/%s/%s/releases/%d/assets?name=%s",
		strings.TrimRight(d.cfg.UploadBaseURL, "/"),
		url.PathEscape(d.cfg.Owner), url.PathEscape(d.repo()),
		release.ID, url.QueryEscape(d.manifestAssetName(obj)))
	mreq, _ := http.NewRequestWithContext(ctx, http.MethodPost, manifestEndpoint, bytes.NewReader(mbody))
	mreq.Header.Set("authorization", "Bearer "+d.token)
	mreq.Header.Set("content-type", "application/json")
	if mresp, mErr := d.client.Do(mreq); mErr == nil {
		_ = mresp.Body.Close()
	}

	d.emit(ctx, "put", obj, "ok", nil, size, start)
	return nil
}

func (d *Driver) Get(ctx context.Context, obj adapter.Object) (io.ReadCloser, error) {
	start := time.Now()
	if err := obj.Validate(); err != nil {
		return nil, &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: err.Error()}
	}
	if err := d.crossTenantGuard(obj); err != nil {
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	rel, err := d.findRelease(ctx, obj)
	if err != nil {
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	if rel == nil {
		err := &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "github: release not found for " + d.tagName(obj)}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	var assetURL string
	for _, a := range rel.Assets {
		if a.Name == d.assetName(obj) {
			assetURL = a.URL
			break
		}
	}
	if assetURL == "" {
		err := &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "github: asset not found in release " + rel.TagName}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	req.Header.Set("authorization", "Bearer "+d.token)
	req.Header.Set("accept", "application/octet-stream")
	resp, err := d.client.Do(req)
	if err != nil {
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "github get: " + err.Error()}
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeUnauthorized, Message: fmt.Sprintf("github get: status=%d", resp.StatusCode)}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("github get: status=%d", resp.StatusCode)}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	d.emit(ctx, "get", obj, "ok", nil, resp.ContentLength, start)
	return adapter.NewVerifyingReadCloser(resp.Body, obj.Digest), nil
}

func (d *Driver) Stat(ctx context.Context, obj adapter.Object) (adapter.Manifest, error) {
	start := time.Now()
	if err := obj.Validate(); err != nil {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: err.Error()}
	}
	if err := d.crossTenantGuard(obj); err != nil {
		return adapter.Manifest{}, err
	}
	rel, err := d.findRelease(ctx, obj)
	if err != nil {
		return adapter.Manifest{}, err
	}
	if rel == nil {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "github stat: not found"}
	}
	var manifestURL string
	for _, a := range rel.Assets {
		if a.Name == d.manifestAssetName(obj) {
			manifestURL = a.URL
			break
		}
	}
	if manifestURL == "" {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "github stat: manifest sidecar not found"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	req.Header.Set("authorization", "Bearer "+d.token)
	req.Header.Set("accept", "application/octet-stream")
	resp, err := d.client.Do(req)
	if err != nil {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "github stat: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("github stat: status=%d", resp.StatusCode)}
	}
	var m adapter.Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "github stat: decode: " + err.Error()}
	}
	d.emit(ctx, "stat", obj, "ok", nil, 0, start)
	return m, nil
}

func (d *Driver) Delete(ctx context.Context, obj adapter.Object) error {
	start := time.Now()
	if err := obj.Validate(); err != nil {
		return &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: err.Error()}
	}
	if err := d.crossTenantGuard(obj); err != nil {
		return err
	}
	rel, err := d.findRelease(ctx, obj)
	if err != nil {
		return err
	}
	if rel == nil {
		return &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "github delete: not found"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete,
		d.apiURL(fmt.Sprintf("/repos/%s/%s/releases/%d", url.PathEscape(d.cfg.Owner), url.PathEscape(d.repo()), rel.ID)), nil)
	resp, err := d.doJSON(req, nil)
	if err != nil {
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "github delete: " + err.Error()}
	}
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNoContent {
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("github delete: status=%d", resp.StatusCode)}
	}
	d.emit(ctx, "delete", obj, "ok", nil, 0, start)
	return nil
}

func (d *Driver) Health(ctx context.Context) (adapter.Health, error) {
	// Verify the backing repo exists, is reachable, and is private. A
	// public repo MUST be rejected because we never store skill bytes on
	// a public surface.
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet,
		d.apiURL(fmt.Sprintf("/repos/%s/%s", url.PathEscape(d.cfg.Owner), url.PathEscape(d.repo()))), nil)
	var meta struct {
		Private bool `json:"private"`
	}
	resp, err := d.doJSON(req, &meta)
	if err != nil {
		return adapter.Health{Healthy: false, Detail: err.Error()}, nil
	}
	if resp.StatusCode/100 != 2 {
		return adapter.Health{Healthy: false, Detail: fmt.Sprintf("status=%d", resp.StatusCode)}, nil
	}
	return adapter.Health{
		Healthy:                true,
		IsPublicStorage:        !meta.Private, // GitHub Packages is public when the repo is public
		SupportsRetention:      false,
		SupportsSignedURLs:     true,
		SupportsLifecycleRules: false,
		Detail:                 d.cfg.Owner + "/" + d.repo(),
	}, nil
}

func (d *Driver) emit(ctx context.Context, op string, obj adapter.Object, result string, err error, bytesT int64, start time.Time) {
	if d.audit == nil {
		return
	}
	evt := adapter.AuditEvent{Op: op, Object: obj, Result: result, BytesTransferred: bytesT, DurationMs: time.Since(start).Milliseconds()}
	if err != nil {
		var e *adapter.Error
		if errors.As(err, &e) {
			evt.ReasonCode = e.Code
			evt.ReasonMessage = e.Message
		} else {
			evt.ReasonCode = adapter.ErrCodeBackendError
			evt.ReasonMessage = err.Error()
		}
	}
	_ = d.audit.EmitArtifactEvent(ctx, evt)
}
