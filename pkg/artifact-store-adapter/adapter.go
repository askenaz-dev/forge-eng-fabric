// Package adapter provides the artifact-store seam used by Forge to publish
// and fetch skill artifacts. The platform stores only the digest, signature
// and adapter pointer on the registry asset row; the bytes themselves live
// in a tenant-configured enterprise artifact store (Nexus, JFrog Artifactory,
// GitHub Packages private, AWS CodeArtifact).
//
// The interface is small on purpose: the registry only ever needs to put
// new immutable versions, fetch them with digest verification, stat them
// for capability/version checks, delete them (administrative only), and
// probe driver health for capability flags. Anything richer (retention
// policies, multi-region replication) is gated on the capability flags
// returned by Health.
//
// Two invariants every driver MUST uphold:
//  1. (AssetID, Version) is immutable. A second Put with the same key MUST
//     return ErrCodeImmutable; the digest of the existing version MUST be
//     verifiable by callers via Stat.
//  2. Per-Tenant isolation. A Tenant MUST NOT be able to read or list
//     another Tenant's artifacts even when the underlying backend is
//     shared. Drivers enforce this by per-Tenant repository or path prefix
//     and Tenant-scoped credentials.
package adapter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// Object identifies a single artifact version. The fields are the minimum
// the registry needs to map an asset row to a backend location: digest is
// authoritative for content addressing, the (TenantID, AssetID, Version)
// triple is authoritative for naming.
type Object struct {
	TenantID string
	AssetID  string
	Version  string
	Digest   string // sha256:<hex64>
}

// Validate ensures the Object can be safely used by drivers. Drivers may
// add backend-specific constraints on top.
func (o Object) Validate() error {
	if o.TenantID == "" {
		return errors.New("adapter: tenant_id required")
	}
	if o.AssetID == "" {
		return errors.New("adapter: asset_id required")
	}
	if o.Version == "" {
		return errors.New("adapter: version required")
	}
	if o.Digest == "" {
		return errors.New("adapter: digest required")
	}
	return nil
}

// Manifest is what Stat returns: the metadata captured at publish time plus
// the canonical digest the registry can re-verify against the asset row.
type Manifest struct {
	AssetID       string    `json:"asset_id"`
	Version       string    `json:"version"`
	Digest        string    `json:"digest"`
	SizeBytes     int64     `json:"size_bytes"`
	SignatureID   string    `json:"signature_id,omitempty"`
	AttestationID string    `json:"attestation_id,omitempty"`
	UploadedAt    time.Time `json:"uploaded_at"`
}

// Health is the result of a driver Health probe. The capability flags
// gate optional features in the registry — e.g. retention policies are
// only offered when `SupportsRetention=true`.
//
// IsPublicStorage=true is a terminal misconfiguration: skill bytes must
// never be stored in a publicly accessible backend. The binding layer
// refuses to construct an adapter whose Health reports it.
//
// IsPublicOrigin=true records that the asset originated from a public
// registry (e.g. npm, GitHub public packages). Public-origin assets are
// allowed when stored in a private backend (IsPublicStorage=false); the
// mirror flow sets this flag to drive the public-origin mirror pipeline.
type Health struct {
	Healthy                bool   `json:"healthy"`
	IsPublicOrigin         bool   `json:"is_public_origin"`  // asset originated from a public registry
	IsPublicStorage        bool   `json:"is_public_storage"` // backend itself is publicly accessible
	SupportsRetention      bool   `json:"supports_retention"`
	SupportsSignedURLs     bool   `json:"supports_signed_urls"`
	SupportsLifecycleRules bool   `json:"supports_lifecycle_rules"`
	Detail                 string `json:"detail,omitempty"`
}

// ErrCode is the structured fault code surfaced to upstream callers; it
// maps to the matching error strings in the skill-artifact-store spec.
type ErrCode string

const (
	ErrCodeImmutable        ErrCode = "version_immutable"
	ErrCodeDigestMismatch   ErrCode = "digest_mismatch"
	ErrCodeNotFound         ErrCode = "not_found"
	ErrCodeCrossTenant      ErrCode = "cross_tenant_read_denied"
	ErrCodeUnauthorized     ErrCode = "unauthorized"
	ErrCodePublicBackend    ErrCode = "public_backend_disallowed"
	ErrCodeBackendError     ErrCode = "backend_error"
	ErrCodeCapabilityMissing ErrCode = "backend_capability_missing"
	ErrCodeInvalidInput     ErrCode = "invalid_input"
)

// Error wraps a backend or contract failure with a Forge-canonical fault
// code. Drivers MUST return *Error for any code that callers branch on;
// transient infra errors may be returned as plain `error` values.
type Error struct {
	Code    ErrCode
	Message string
	Wrapped error
}

func (e *Error) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Wrapped)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Wrapped }

// IsCode reports whether err is an *Error with the given code; convenience
// for callers that need to branch on adapter codes.
func IsCode(err error, code ErrCode) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}

// AuditEvent is emitted on every adapter operation. The registry's audit
// pipeline ingests these into the standard event stream so the per-asset
// observability spec can correlate publishes with downstream invocations.
type AuditEvent struct {
	Op               string  `json:"op"` // "put", "get", "stat", "delete", "health"
	Actor            string  `json:"actor,omitempty"`
	CorrelationID    string  `json:"correlation_id,omitempty"`
	Object           Object  `json:"object"`
	Result           string  `json:"result"` // "ok" | "error"
	ReasonCode       ErrCode `json:"reason_code,omitempty"`
	ReasonMessage    string  `json:"reason_message,omitempty"`
	BytesTransferred int64   `json:"bytes_transferred,omitempty"`
	DurationMs       int64   `json:"duration_ms"`
}

// AuditSink is the seam adapters call to emit audit events. The default
// implementation in the registry forwards to Kafka; tests provide a
// capture sink.
type AuditSink interface {
	EmitArtifactEvent(ctx context.Context, e AuditEvent) error
}

// NoOpAuditSink discards events. Convenient default for unit tests of the
// adapter contract that do not care about audit emission.
type NoOpAuditSink struct{}

func (NoOpAuditSink) EmitArtifactEvent(_ context.Context, _ AuditEvent) error { return nil }

// Adapter is the contract every driver implements. Drivers SHOULD be safe
// for concurrent use by multiple goroutines.
type Adapter interface {
	// Put uploads a new artifact version. The reader is consumed exactly
	// once. Implementations MUST refuse to overwrite an existing
	// (TenantID, AssetID, Version) with ErrCodeImmutable, and MUST verify
	// the uploaded bytes hash to Digest after the upload completes.
	Put(ctx context.Context, obj Object, content io.Reader, size int64, manifest Manifest) error

	// Get returns a reader over the artifact bytes. The reader MUST hash
	// streamed bytes and return ErrCodeDigestMismatch on close if the
	// observed digest does not match Object.Digest.
	Get(ctx context.Context, obj Object) (io.ReadCloser, error)

	// Stat returns the stored manifest for the artifact without
	// transferring the bytes.
	Stat(ctx context.Context, obj Object) (Manifest, error)

	// Delete removes the artifact. Drivers MAY refuse deletion if the
	// backend enforces immutability at storage layer; in that case they
	// return ErrCodeImmutable.
	Delete(ctx context.Context, obj Object) error

	// Health probes the backend. The capability flags returned here are
	// the source of truth that the registry consults when deciding which
	// optional features to offer (retention policies, signed URLs, etc.).
	Health(ctx context.Context) (Health, error)

	// Backend returns the canonical backend identifier (e.g. "nexus",
	// "artifactory") for audit and metrics labelling.
	Backend() string
}

// SecretFetcher is the seam drivers use to resolve the credential ref
// stored on artifact_store_binding into a usable secret value. The
// concrete implementation in production talks to Vault/IAM/Secrets
// Manager; the test implementation returns a static map.
type SecretFetcher interface {
	Fetch(ctx context.Context, ref string) ([]byte, error)
}

// StaticSecretFetcher is a test helper that resolves a fixed map.
type StaticSecretFetcher map[string][]byte

func (s StaticSecretFetcher) Fetch(_ context.Context, ref string) ([]byte, error) {
	v, ok := s[ref]
	if !ok {
		return nil, &Error{Code: ErrCodeUnauthorized, Message: "no secret bound for ref=" + ref}
	}
	return v, nil
}
