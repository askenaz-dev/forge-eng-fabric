// Package nexus implements the artifact-store adapter for Sonatype Nexus
// Repository (raw repositories). The driver maps the Forge content model
// onto Nexus paths:
//
//	repo := per-tenant raw repository named  "forge-skills-{tenant_id}"
//	path := "{asset_id}/{version}/{asset_id}-{version}.tar.zst"
//	manifest := the same path with a ".manifest.json" suffix
//
// Per-Tenant isolation is enforced by repository scoping: a Tenant's
// adapter is configured with credentials that grant access only to its
// own repository, and Get/Stat/Delete refuse to look outside the
// configured RepoPrefix. Cross-tenant read attempts return
// ErrCodeCrossTenant.
//
// Immutability: a second PUT to an existing path is rejected at the
// Nexus level (raw repos with `strictContentTypeValidation=true` and
// `writePolicy=ALLOW_ONCE`); the driver double-checks by doing a HEAD
// before PUT and returning ErrCodeImmutable on collision.
package nexus

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

// Config is the typed shape of BindingConfig.Settings for nexus.
type Config struct {
	BaseURL        string `json:"base_url"`
	RepoPrefix     string `json:"repo_prefix"` // default "forge-skills"
	Username       string `json:"username"`    // optional; when empty, only credential_ref is used
	ContentType    string `json:"content_type"`
	HealthEndpoint string `json:"health_endpoint"` // default /service/rest/v1/status
}

// Driver is the Nexus implementation of adapter.Adapter.
type Driver struct {
	cfg     Config
	tenant  string
	authHdr string
	client  *http.Client
	audit   adapter.AuditSink
}

func init() {
	adapter.RegisterDriver(adapter.BackendNexus, build)
}

func build(ctx context.Context, cfg adapter.BindingConfig, deps adapter.Deps) (adapter.Adapter, error) {
	var typed Config
	if err := adapter.DecodeSettings(cfg.Settings, &typed); err != nil {
		return nil, err
	}
	if typed.BaseURL == "" {
		return nil, &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: "nexus: base_url required"}
	}
	if typed.RepoPrefix == "" {
		typed.RepoPrefix = "forge-skills"
	}
	if typed.ContentType == "" {
		typed.ContentType = "application/octet-stream"
	}
	if typed.HealthEndpoint == "" {
		typed.HealthEndpoint = "/service/rest/v1/status"
	}

	secret, err := deps.SecretFetcher.Fetch(ctx, cfg.CredentialRef)
	if err != nil {
		return nil, err
	}
	username := typed.Username
	password := string(secret)
	// If credentials are provided as "user:pass" pair, split.
	if username == "" && bytes.Contains(secret, []byte(":")) {
		parts := bytes.SplitN(secret, []byte(":"), 2)
		username = string(parts[0])
		password = string(parts[1])
	}
	authHdr := ""
	if username != "" || password != "" {
		token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		authHdr = "Basic " + token
	}

	return &Driver{
		cfg:     typed,
		tenant:  cfg.TenantID,
		authHdr: authHdr,
		client:  &http.Client{Timeout: 60 * time.Second},
		audit:   deps.AuditSink,
	}, nil
}

func (d *Driver) Backend() string { return adapter.BackendNexus }

func (d *Driver) repo() string {
	return d.cfg.RepoPrefix + "-" + d.tenant
}

func (d *Driver) artifactPath(obj adapter.Object) string {
	return path.Join("repository", d.repo(), obj.AssetID, obj.Version, fmt.Sprintf("%s-%s.tar.zst", obj.AssetID, obj.Version))
}

func (d *Driver) manifestPath(obj adapter.Object) string {
	return d.artifactPath(obj) + ".manifest.json"
}

func (d *Driver) url(p string) string {
	return strings.TrimRight(d.cfg.BaseURL, "/") + "/" + strings.TrimLeft(p, "/")
}

// crossTenantGuard returns ErrCodeCrossTenant if obj.TenantID does not match
// the tenant this driver was constructed for. Drivers are always
// tenant-scoped; a mismatched call indicates a programmer error or an
// authorization leak upstream.
func (d *Driver) crossTenantGuard(obj adapter.Object) error {
	if obj.TenantID != d.tenant {
		return &adapter.Error{
			Code:    adapter.ErrCodeCrossTenant,
			Message: fmt.Sprintf("driver bound to tenant=%s; refused access to object owned by tenant=%s", d.tenant, obj.TenantID),
		}
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

	// HEAD first: refuse to overwrite an existing version.
	headReq, _ := http.NewRequestWithContext(ctx, http.MethodHead, d.url(d.artifactPath(obj)), nil)
	d.setAuth(headReq)
	if resp, err := d.client.Do(headReq); err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			err := &adapter.Error{Code: adapter.ErrCodeImmutable, Message: fmt.Sprintf("nexus: %s@%s already exists", obj.AssetID, obj.Version)}
			d.emit(ctx, "put", obj, "error", err, 0, start)
			return err
		}
	}

	// Hash bytes as they upload so we can verify against the manifest digest.
	hashed := adapter.NewHashingReader(content)
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, d.url(d.artifactPath(obj)), hashed)
	if err != nil {
		return err
	}
	d.setAuth(putReq)
	putReq.Header.Set("content-type", d.cfg.ContentType)
	if size > 0 {
		putReq.ContentLength = size
	}
	resp, err := d.client.Do(putReq)
	if err != nil {
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "nexus put: " + err.Error(), Wrapped: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		// Nexus returns 400/422 for write-policy violations on existing assets.
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
			err := &adapter.Error{Code: adapter.ErrCodeImmutable, Message: fmt.Sprintf("nexus: write rejected (%d): %s", resp.StatusCode, string(body))}
			d.emit(ctx, "put", obj, "error", err, 0, start)
			return err
		}
		err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("nexus put status=%d body=%s", resp.StatusCode, string(body))}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}

	// Verify the uploaded digest matches what the caller declared. If it
	// doesn't, the bytes are bad — record the failure but the bytes are
	// already in Nexus. The registry will refuse to record this version,
	// and an operator should run Delete to clean up. We do not auto-delete
	// here because that path is racy with replicated backends.
	if observed := hashed.Sum(); observed != obj.Digest {
		err := &adapter.Error{Code: adapter.ErrCodeDigestMismatch, Message: fmt.Sprintf("nexus: uploaded digest %s does not match expected %s", observed, obj.Digest)}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}

	// Write the per-version manifest sidecar so Stat does not need a
	// second metadata system. Manifest mirrors what the registry already
	// captured but lets the adapter answer Stat without a registry call.
	manifest.AssetID = obj.AssetID
	manifest.Version = obj.Version
	manifest.Digest = obj.Digest
	if manifest.UploadedAt.IsZero() {
		manifest.UploadedAt = time.Now().UTC()
	}
	manifestBytes, _ := json.Marshal(manifest)
	mreq, _ := http.NewRequestWithContext(ctx, http.MethodPut, d.url(d.manifestPath(obj)), bytes.NewReader(manifestBytes))
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
		return nil, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "nexus get: " + err.Error(), Wrapped: err}
	}
	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "nexus: " + obj.AssetID + "@" + obj.Version + " not found"}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeUnauthorized, Message: fmt.Sprintf("nexus get: status=%d", resp.StatusCode)}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("nexus get: status=%d", resp.StatusCode)}
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
		d.emit(ctx, "stat", obj, "error", err, 0, start)
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "nexus stat: " + err.Error(), Wrapped: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		err := &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "nexus stat: manifest sidecar missing for " + obj.AssetID + "@" + obj.Version}
		d.emit(ctx, "stat", obj, "error", err, 0, start)
		return adapter.Manifest{}, err
	}
	if resp.StatusCode/100 != 2 {
		err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("nexus stat: status=%d", resp.StatusCode)}
		d.emit(ctx, "stat", obj, "error", err, 0, start)
		return adapter.Manifest{}, err
	}
	var m adapter.Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		d.emit(ctx, "stat", obj, "error", err, 0, start)
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "nexus stat: decode manifest: " + err.Error()}
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
		d.emit(ctx, "delete", obj, "error", err, 0, start)
		return err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, d.url(d.artifactPath(obj)), nil)
	d.setAuth(req)
	resp, err := d.client.Do(req)
	if err != nil {
		d.emit(ctx, "delete", obj, "error", err, 0, start)
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "nexus delete: " + err.Error(), Wrapped: err}
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		err := &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "nexus delete: not found"}
		d.emit(ctx, "delete", obj, "error", err, 0, start)
		return err
	}
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNoContent {
		err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("nexus delete: status=%d", resp.StatusCode)}
		d.emit(ctx, "delete", obj, "error", err, 0, start)
		return err
	}
	// Best-effort manifest cleanup.
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
		IsPublicStorage:        false, // Nexus by default requires auth; the operator must configure a private repo
		SupportsRetention:      true,
		SupportsSignedURLs:     false,
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
	evt := adapter.AuditEvent{
		Op:               op,
		Object:           obj,
		Result:           result,
		BytesTransferred: bytesT,
		DurationMs:       time.Since(start).Milliseconds(),
	}
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
