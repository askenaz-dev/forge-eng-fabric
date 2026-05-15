// Package artifactory implements the artifact-store adapter for JFrog
// Artifactory generic repositories. Per-Tenant isolation uses repo-per-
// tenant scoping; the artifact path mirrors the Nexus driver so the
// pkg/skill-packager output is bytes-identical across backends.
//
// API: Artifactory REST v1 (`PUT /artifactory/{repo}/{path}` for
// uploads, `GET /artifactory/{repo}/{path}` for downloads). Auth is
// either basic (user+token) or Bearer (identity-token), determined by
// the credential blob format.
package artifactory

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	adapter "github.com/forge-eng-fabric/pkg/artifact-store-adapter"
)

type Config struct {
	BaseURL        string `json:"base_url"`
	RepoPrefix     string `json:"repo_prefix"`
	Username       string `json:"username"`
	ContentType    string `json:"content_type"`
	HealthEndpoint string `json:"health_endpoint"`
}

type Driver struct {
	cfg     Config
	tenant  string
	authHdr string
	client  *http.Client
	audit   adapter.AuditSink
}

func init() {
	adapter.RegisterDriver(adapter.BackendArtifactory, build)
}

func build(ctx context.Context, cfg adapter.BindingConfig, deps adapter.Deps) (adapter.Adapter, error) {
	var typed Config
	if err := adapter.DecodeSettings(cfg.Settings, &typed); err != nil {
		return nil, err
	}
	if typed.BaseURL == "" {
		return nil, &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: "artifactory: base_url required"}
	}
	if typed.RepoPrefix == "" {
		typed.RepoPrefix = "forge-skills"
	}
	if typed.ContentType == "" {
		typed.ContentType = "application/octet-stream"
	}
	if typed.HealthEndpoint == "" {
		typed.HealthEndpoint = "/artifactory/api/system/ping"
	}
	secret, err := deps.SecretFetcher.Fetch(ctx, cfg.CredentialRef)
	if err != nil {
		return nil, err
	}
	authHdr := ""
	// Identity tokens are opaque strings; basic auth carries "user:pass".
	if bytes.Contains(secret, []byte(":")) {
		parts := bytes.SplitN(secret, []byte(":"), 2)
		token := base64.StdEncoding.EncodeToString([]byte(string(parts[0]) + ":" + string(parts[1])))
		authHdr = "Basic " + token
	} else if typed.Username != "" {
		token := base64.StdEncoding.EncodeToString([]byte(typed.Username + ":" + string(secret)))
		authHdr = "Basic " + token
	} else {
		authHdr = "Bearer " + string(secret)
	}
	return &Driver{
		cfg:     typed,
		tenant:  cfg.TenantID,
		authHdr: authHdr,
		client:  &http.Client{Timeout: 60 * time.Second},
		audit:   deps.AuditSink,
	}, nil
}

func (d *Driver) Backend() string { return adapter.BackendArtifactory }

func (d *Driver) repo() string {
	return d.cfg.RepoPrefix + "-" + d.tenant
}

func (d *Driver) artifactPath(obj adapter.Object) string {
	return path.Join("artifactory", d.repo(), obj.AssetID, obj.Version, fmt.Sprintf("%s-%s.tar.zst", obj.AssetID, obj.Version))
}

func (d *Driver) manifestPath(obj adapter.Object) string {
	return d.artifactPath(obj) + ".manifest.json"
}

func (d *Driver) url(p string) string {
	return strings.TrimRight(d.cfg.BaseURL, "/") + "/" + strings.TrimLeft(p, "/")
}

func (d *Driver) crossTenantGuard(obj adapter.Object) error {
	if obj.TenantID != d.tenant {
		return &adapter.Error{Code: adapter.ErrCodeCrossTenant, Message: fmt.Sprintf("driver bound to tenant=%s; refused tenant=%s", d.tenant, obj.TenantID)}
	}
	return nil
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
	// Artifactory raw repos: a HEAD-then-PUT pattern double-checks the
	// immutability rule that we also configure at the repo level.
	headReq, _ := http.NewRequestWithContext(ctx, http.MethodHead, d.url(d.artifactPath(obj)), nil)
	d.setAuth(headReq)
	if resp, err := d.client.Do(headReq); err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			err := &adapter.Error{Code: adapter.ErrCodeImmutable, Message: fmt.Sprintf("artifactory: %s@%s already exists", obj.AssetID, obj.Version)}
			d.emit(ctx, "put", obj, "error", err, 0, start)
			return err
		}
	}
	hashed := adapter.NewHashingReader(content)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, d.url(d.artifactPath(obj)), hashed)
	d.setAuth(req)
	req.Header.Set("content-type", d.cfg.ContentType)
	// Artifactory understands X-Checksum-Sha256 to short-circuit upload if
	// the digest already exists; passing it eagerly lets the server reject
	// mismatches earlier than the post-upload digest verification.
	req.Header.Set("X-Checksum-Sha256", strings.TrimPrefix(obj.Digest, "sha256:"))
	if size > 0 {
		req.ContentLength = size
	}
	resp, err := d.client.Do(req)
	if err != nil {
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "artifactory put: " + err.Error(), Wrapped: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		err := &adapter.Error{Code: adapter.ErrCodeImmutable, Message: "artifactory: version exists (409)"}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("artifactory put: status=%d body=%s", resp.StatusCode, string(body))}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}
	if observed := hashed.Sum(); observed != obj.Digest {
		err := &adapter.Error{Code: adapter.ErrCodeDigestMismatch, Message: fmt.Sprintf("artifactory: uploaded digest %s does not match expected %s", observed, obj.Digest)}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}
	// Write manifest sidecar.
	manifest.AssetID = obj.AssetID
	manifest.Version = obj.Version
	manifest.Digest = obj.Digest
	if manifest.UploadedAt.IsZero() {
		manifest.UploadedAt = time.Now().UTC()
	}
	body, _ := json.Marshal(manifest)
	mreq, _ := http.NewRequestWithContext(ctx, http.MethodPut, d.url(d.manifestPath(obj)), bytes.NewReader(body))
	d.setAuth(mreq)
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
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, d.url(d.artifactPath(obj)), nil)
	d.setAuth(req)
	resp, err := d.client.Do(req)
	if err != nil {
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "artifactory get: " + err.Error(), Wrapped: err}
	}
	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "artifactory: not found"}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeUnauthorized, Message: fmt.Sprintf("artifactory get: status=%d", resp.StatusCode)}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("artifactory get: status=%d", resp.StatusCode)}
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
		d.emit(ctx, "stat", obj, "error", err, 0, start)
		return adapter.Manifest{}, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, d.url(d.manifestPath(obj)), nil)
	d.setAuth(req)
	resp, err := d.client.Do(req)
	if err != nil {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "artifactory stat: " + err.Error(), Wrapped: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "artifactory stat: not found"}
	}
	if resp.StatusCode/100 != 2 {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("artifactory stat: status=%d", resp.StatusCode)}
	}
	var m adapter.Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "artifactory stat: decode: " + err.Error()}
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
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, d.url(d.artifactPath(obj)), nil)
	d.setAuth(req)
	resp, err := d.client.Do(req)
	if err != nil {
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "artifactory delete: " + err.Error(), Wrapped: err}
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "artifactory delete: not found"}
	}
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNoContent {
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("artifactory delete: status=%d", resp.StatusCode)}
	}
	mreq, _ := http.NewRequestWithContext(ctx, http.MethodDelete, d.url(d.manifestPath(obj)), nil)
	d.setAuth(mreq)
	if mresp, mErr := d.client.Do(mreq); mErr == nil {
		_ = mresp.Body.Close()
	}
	d.emit(ctx, "delete", obj, "ok", nil, 0, start)
	return nil
}

func (d *Driver) Health(ctx context.Context) (adapter.Health, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, d.url(d.cfg.HealthEndpoint), nil)
	d.setAuth(req)
	resp, err := d.client.Do(req)
	if err != nil {
		return adapter.Health{Healthy: false, Detail: err.Error()}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return adapter.Health{Healthy: false, Detail: fmt.Sprintf("status=%d", resp.StatusCode)}, nil
	}
	return adapter.Health{
		Healthy:                true,
		IsPublicStorage:        false,
		SupportsRetention:      true,
		SupportsSignedURLs:     true,
		SupportsLifecycleRules: true,
	}, nil
}

func (d *Driver) setAuth(req *http.Request) {
	if d.authHdr != "" {
		req.Header.Set("authorization", d.authHdr)
	}
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
