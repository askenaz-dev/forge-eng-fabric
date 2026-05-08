// Package cosign provides Verify and VerifyAttestation primitives used by
// the deploy-orchestrator's image-verification stage. It models the
// "Cosign signature verification" + "SLSA attestation verification" + "Rekor
// lookup" requirements from the `image-verification-at-deploy` spec.
//
// The default implementation here is policy-only (it inspects metadata
// submitted alongside the image) so that tests and local dev can run
// without Sigstore. A production implementation wires this to the cosign
// CLI or the cosign Go libraries.
package cosign

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailed  Outcome = "failed"
)

type FailReason string

const (
	ReasonIdentityMismatch FailReason = "identity_mismatch"
	ReasonRekorMissing     FailReason = "rekor_entry_missing"
	ReasonDigestMismatch   FailReason = "digest_mismatch"
	ReasonUnsigned         FailReason = "unsigned_image"
	ReasonAttestationBad   FailReason = "attestation_invalid"
)

// ImageMetadata is the input that callers (orchestrator) hand the verifier.
// It mirrors what the registry plus build-time signing record per image.
type ImageMetadata struct {
	Image                string
	Digest               string
	Signed               bool
	OIDCIssuer           string
	OIDCIdentity         string
	RekorLogIndex        int64
	HasRekorEntry        bool
	AttestationType      string // expected: "slsaprovenance"
	AttestationPredicate map[string]any
	AttestationValid     bool
	BuiltAt              time.Time
}

type RegistryDigestLookup interface {
	LookupExpectedDigest(ctx context.Context, image string) (string, error)
}

// IdentityRule describes the OIDC identity expected for a Workspace.
type IdentityRule struct {
	IssuerExact     string
	IdentityRegexp  *regexp.Regexp
	DataClassification string // "confidential" / "restricted" require private rekor
}

type WorkspaceIdentities struct {
	rules map[string]IdentityRule
}

func NewWorkspaceIdentities() *WorkspaceIdentities {
	return &WorkspaceIdentities{rules: map[string]IdentityRule{}}
}

func (w *WorkspaceIdentities) Configure(workspaceID string, rule IdentityRule) {
	w.rules[workspaceID] = rule
}

func (w *WorkspaceIdentities) Get(workspaceID string) (IdentityRule, bool) {
	r, ok := w.rules[workspaceID]
	return r, ok
}

type Verifier struct {
	Identities *WorkspaceIdentities
	Registry   RegistryDigestLookup
}

func NewVerifier(identities *WorkspaceIdentities, registry RegistryDigestLookup) *Verifier {
	if identities == nil {
		identities = NewWorkspaceIdentities()
	}
	return &Verifier{Identities: identities, Registry: registry}
}

type Result struct {
	Outcome    Outcome    `json:"outcome"`
	Reason     FailReason `json:"reason,omitempty"`
	Detail     string     `json:"detail,omitempty"`
	Identity   string     `json:"identity,omitempty"`
	RekorIndex int64      `json:"rekor_index,omitempty"`
	Digest     string     `json:"digest,omitempty"`
}

// Verify runs the signature + identity + rekor checks.
func (v *Verifier) Verify(ctx context.Context, workspaceID string, m ImageMetadata) Result {
	if !m.Signed {
		return Result{Outcome: OutcomeFailed, Reason: ReasonUnsigned, Detail: "image is unsigned"}
	}
	if rule, ok := v.Identities.Get(workspaceID); ok {
		if rule.IssuerExact != "" && rule.IssuerExact != m.OIDCIssuer {
			return Result{Outcome: OutcomeFailed, Reason: ReasonIdentityMismatch, Detail: "issuer mismatch", Identity: m.OIDCIdentity}
		}
		if rule.IdentityRegexp != nil && !rule.IdentityRegexp.MatchString(m.OIDCIdentity) {
			return Result{Outcome: OutcomeFailed, Reason: ReasonIdentityMismatch, Detail: "identity does not match expected pattern", Identity: m.OIDCIdentity}
		}
	}
	if !m.HasRekorEntry {
		return Result{Outcome: OutcomeFailed, Reason: ReasonRekorMissing, Detail: "missing rekor entry"}
	}
	return Result{Outcome: OutcomeSuccess, Identity: m.OIDCIdentity, RekorIndex: m.RekorLogIndex, Digest: m.Digest}
}

// VerifyAttestation enforces SLSA provenance + digest match against the
// registry record.
func (v *Verifier) VerifyAttestation(ctx context.Context, m ImageMetadata) Result {
	if m.AttestationType != "slsaprovenance" {
		return Result{Outcome: OutcomeFailed, Reason: ReasonAttestationBad, Detail: "attestation_type=" + m.AttestationType}
	}
	if !m.AttestationValid {
		return Result{Outcome: OutcomeFailed, Reason: ReasonAttestationBad, Detail: "attestation invalid"}
	}
	if v.Registry != nil {
		expected, err := v.Registry.LookupExpectedDigest(ctx, m.Image)
		if err != nil {
			return Result{Outcome: OutcomeFailed, Reason: ReasonAttestationBad, Detail: "registry lookup: " + err.Error()}
		}
		if expected != "" && !strings.EqualFold(expected, m.Digest) {
			return Result{Outcome: OutcomeFailed, Reason: ReasonDigestMismatch, Detail: "registry expected " + expected + " got " + m.Digest, Digest: m.Digest}
		}
	}
	return Result{Outcome: OutcomeSuccess, Digest: m.Digest}
}

// Combined runs Verify + VerifyAttestation; first failure wins.
func (v *Verifier) Combined(ctx context.Context, workspaceID string, m ImageMetadata) Result {
	if r := v.Verify(ctx, workspaceID, m); r.Outcome != OutcomeSuccess {
		return r
	}
	return v.VerifyAttestation(ctx, m)
}

// StaticDigestLookup implements the RegistryDigestLookup interface from a
// static map. Used in tests.
type StaticDigestLookup map[string]string

func (s StaticDigestLookup) LookupExpectedDigest(_ context.Context, image string) (string, error) {
	d, ok := s[image]
	if !ok {
		return "", errors.New("no expected digest for image=" + image)
	}
	return d, nil
}
