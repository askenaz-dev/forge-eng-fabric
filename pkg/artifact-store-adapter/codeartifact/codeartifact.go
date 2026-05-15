// Package codeartifact implements the artifact-store adapter for AWS
// CodeArtifact (binding key `codeartifact`). The driver uses the
// "generic" package format so it can store arbitrary tar.zst skill
// bytes without coupling to npm/maven/pip conventions.
//
// Per-Tenant isolation is enforced by mapping each Tenant to its own
// CodeArtifact repository under a shared domain — repositories in
// CodeArtifact are independently access-controlled and the credentials
// the binding holds grant access only to the Tenant's own repo.
package codeartifact

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
	Region       string `json:"region"`
	Domain       string `json:"domain"`        // CodeArtifact domain
	DomainOwner  string `json:"domain_owner"`  // AWS account id owning the domain
	RepoPrefix   string `json:"repo_prefix"`   // per-tenant repo prefix; default "forge-skills"
	Namespace    string `json:"namespace"`     // default "forge.skills"
	Endpoint     string `json:"endpoint"`      // override; default https://codeartifact.<region>.amazonaws.com
	Format       string `json:"format"`        // CodeArtifact format; default "generic"
}

type Driver struct {
	cfg    Config
	tenant string
	signer *Signer
	client *http.Client
	audit  adapter.AuditSink
}

func init() {
	adapter.RegisterDriver(adapter.BackendCodeArtifact, build)
}

// secretFormat decodes the credential blob fetched from the SecretFetcher.
// Two formats accepted:
//  1. JSON  {"access_key_id": "...", "secret_access_key": "...", "session_token": "..."}
//  2. Three colon-separated fields:  access_key_id:secret_access_key[:session_token]
type secretFormat struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token,omitempty"`
}

func parseSecret(raw []byte) (Credentials, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return Credentials{}, errors.New("codeartifact: empty credential blob")
	}
	if raw[0] == '{' {
		var sf secretFormat
		if err := json.Unmarshal(raw, &sf); err != nil {
			return Credentials{}, err
		}
		return Credentials{AccessKeyID: sf.AccessKeyID, SecretAccessKey: sf.SecretAccessKey, SessionToken: sf.SessionToken}, nil
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) < 2 {
		return Credentials{}, errors.New("codeartifact: credential blob must be JSON or access_key_id:secret_access_key[:session_token]")
	}
	c := Credentials{AccessKeyID: parts[0], SecretAccessKey: parts[1]}
	if len(parts) >= 3 {
		c.SessionToken = parts[2]
	}
	return c, nil
}

func build(ctx context.Context, cfg adapter.BindingConfig, deps adapter.Deps) (adapter.Adapter, error) {
	var typed Config
	if err := adapter.DecodeSettings(cfg.Settings, &typed); err != nil {
		return nil, err
	}
	if typed.Region == "" {
		return nil, &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: "codeartifact: region required"}
	}
	if typed.Domain == "" {
		return nil, &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: "codeartifact: domain required"}
	}
	if typed.RepoPrefix == "" {
		typed.RepoPrefix = "forge-skills"
	}
	if typed.Namespace == "" {
		typed.Namespace = "forge.skills"
	}
	if typed.Format == "" {
		typed.Format = "generic"
	}
	if typed.Endpoint == "" {
		typed.Endpoint = "https://codeartifact." + typed.Region + ".amazonaws.com"
	}
	secret, err := deps.SecretFetcher.Fetch(ctx, cfg.CredentialRef)
	if err != nil {
		return nil, err
	}
	creds, err := parseSecret(secret)
	if err != nil {
		return nil, &adapter.Error{Code: adapter.ErrCodeUnauthorized, Message: err.Error()}
	}
	return &Driver{
		cfg:    typed,
		tenant: cfg.TenantID,
		signer: &Signer{Creds: creds, Region: typed.Region, Service: "codeartifact"},
		client: &http.Client{Timeout: 60 * time.Second},
		audit:  deps.AuditSink,
	}, nil
}

func (d *Driver) Backend() string { return adapter.BackendCodeArtifact }

func (d *Driver) repo() string {
	return d.cfg.RepoPrefix + "-" + d.tenant
}

func (d *Driver) crossTenantGuard(obj adapter.Object) error {
	if obj.TenantID != d.tenant {
		return &adapter.Error{Code: adapter.ErrCodeCrossTenant, Message: fmt.Sprintf("driver bound to tenant=%s; refused tenant=%s", d.tenant, obj.TenantID)}
	}
	return nil
}

func (d *Driver) assetName(obj adapter.Object) string {
	return fmt.Sprintf("%s-%s.tar.zst", obj.AssetID, obj.Version)
}

func (d *Driver) manifestAssetName(obj adapter.Object) string {
	return d.assetName(obj) + ".manifest.json"
}

// commonQuery builds the canonical query string for the CodeArtifact
// asset endpoints. CodeArtifact expects: domain, domainOwner (optional),
// repository, format, namespace (optional), package, packageVersion.
func (d *Driver) commonQuery(obj adapter.Object, asset string) url.Values {
	q := url.Values{}
	q.Set("domain", d.cfg.Domain)
	if d.cfg.DomainOwner != "" {
		q.Set("domainOwner", d.cfg.DomainOwner)
	}
	q.Set("repository", d.repo())
	q.Set("format", d.cfg.Format)
	if d.cfg.Namespace != "" {
		q.Set("namespace", d.cfg.Namespace)
	}
	q.Set("package", obj.AssetID)
	q.Set("packageVersion", obj.Version)
	if asset != "" {
		q.Set("asset", asset)
	}
	return q
}

func (d *Driver) endpoint(path string, q url.Values) string {
	return strings.TrimRight(d.cfg.Endpoint, "/") + path + "?" + q.Encode()
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
	// CodeArtifact PublishPackageVersion requires the asset SHA256 in
	// a header. Buffer the body so we can both hash and sign it.
	body, err := io.ReadAll(content)
	if err != nil {
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "codeartifact put read: " + err.Error()}
	}
	observed := adapter.DigestSHA256(body)
	if observed != obj.Digest {
		err := &adapter.Error{Code: adapter.ErrCodeDigestMismatch, Message: fmt.Sprintf("codeartifact: uploaded digest %s does not match expected %s", observed, obj.Digest)}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}
	bodyHash, _, _ := HashBody(bytes.NewReader(body))
	q := d.commonQuery(obj, d.assetName(obj))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint("/v1/asset", q), bytes.NewReader(body))
	req.Header.Set("content-type", "application/octet-stream")
	req.Header.Set("x-amz-content-sha256", bodyHash)
	req.Header.Set("x-amz-codeartifact-asset-sha256", strings.TrimPrefix(obj.Digest, "sha256:"))
	d.signer.Sign(req, bodyHash, time.Now())
	resp, err := d.client.Do(req)
	if err != nil {
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "codeartifact put: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		err := &adapter.Error{Code: adapter.ErrCodeImmutable, Message: "codeartifact: version exists"}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}
	if resp.StatusCode/100 != 2 {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("codeartifact put: status=%d body=%s", resp.StatusCode, string(buf))}
		d.emit(ctx, "put", obj, "error", err, 0, start)
		return err
	}

	// Manifest sidecar.
	manifest.AssetID = obj.AssetID
	manifest.Version = obj.Version
	manifest.Digest = obj.Digest
	if manifest.UploadedAt.IsZero() {
		manifest.UploadedAt = time.Now().UTC()
	}
	mbody, _ := json.Marshal(manifest)
	mHash := adapter.DigestSHA256(mbody)
	mBodyHash := strings.TrimPrefix(mHash, "sha256:")
	mq := d.commonQuery(obj, d.manifestAssetName(obj))
	mreq, _ := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint("/v1/asset", mq), bytes.NewReader(mbody))
	mreq.Header.Set("content-type", "application/json")
	mreq.Header.Set("x-amz-content-sha256", mBodyHash)
	mreq.Header.Set("x-amz-codeartifact-asset-sha256", mBodyHash)
	d.signer.Sign(mreq, mBodyHash, time.Now())
	if mresp, mErr := d.client.Do(mreq); mErr == nil {
		_ = mresp.Body.Close()
	}

	d.emit(ctx, "put", obj, "ok", nil, int64(len(body)), start)
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
	q := d.commonQuery(obj, d.assetName(obj))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, d.endpoint("/v1/asset", q), nil)
	d.signer.Sign(req, emptyBodySHA256, time.Now())
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "codeartifact get: " + err.Error()}
	}
	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "codeartifact: not found"}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeUnauthorized, Message: fmt.Sprintf("codeartifact get: status=%d", resp.StatusCode)}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		err := &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("codeartifact get: status=%d", resp.StatusCode)}
		d.emit(ctx, "get", obj, "error", err, 0, start)
		return nil, err
	}
	d.emit(ctx, "get", obj, "ok", nil, resp.ContentLength, start)
	return adapter.NewVerifyingReadCloser(resp.Body, obj.Digest), nil
}

func (d *Driver) Stat(ctx context.Context, obj adapter.Object) (adapter.Manifest, error) {
	if err := obj.Validate(); err != nil {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: err.Error()}
	}
	if err := d.crossTenantGuard(obj); err != nil {
		return adapter.Manifest{}, err
	}
	q := d.commonQuery(obj, d.manifestAssetName(obj))
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, d.endpoint("/v1/asset", q), nil)
	d.signer.Sign(req, emptyBodySHA256, time.Now())
	resp, err := d.client.Do(req)
	if err != nil {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "codeartifact stat: " + err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "codeartifact stat: not found"}
	}
	if resp.StatusCode/100 != 2 {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("codeartifact stat: status=%d", resp.StatusCode)}
	}
	var m adapter.Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return adapter.Manifest{}, &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "codeartifact stat: decode: " + err.Error()}
	}
	return m, nil
}

func (d *Driver) Delete(ctx context.Context, obj adapter.Object) error {
	if err := obj.Validate(); err != nil {
		return &adapter.Error{Code: adapter.ErrCodeInvalidInput, Message: err.Error()}
	}
	if err := d.crossTenantGuard(obj); err != nil {
		return err
	}
	q := d.commonQuery(obj, "")
	q.Set("versions", obj.Version)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint("/v1/package/versions/delete", q), nil)
	d.signer.Sign(req, emptyBodySHA256, time.Now())
	resp, err := d.client.Do(req)
	if err != nil {
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: "codeartifact delete: " + err.Error()}
	}
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return &adapter.Error{Code: adapter.ErrCodeNotFound, Message: "codeartifact delete: not found"}
	}
	if resp.StatusCode/100 != 2 {
		return &adapter.Error{Code: adapter.ErrCodeBackendError, Message: fmt.Sprintf("codeartifact delete: status=%d", resp.StatusCode)}
	}
	return nil
}

func (d *Driver) Health(ctx context.Context) (adapter.Health, error) {
	q := url.Values{}
	q.Set("domain", d.cfg.Domain)
	if d.cfg.DomainOwner != "" {
		q.Set("domainOwner", d.cfg.DomainOwner)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, d.endpoint("/v1/domain", q), nil)
	d.signer.Sign(req, emptyBodySHA256, time.Now())
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
		IsPublicStorage:        false, // CodeArtifact domains are private by design
		SupportsRetention:      false,
		SupportsSignedURLs:     true,
		SupportsLifecycleRules: false,
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
