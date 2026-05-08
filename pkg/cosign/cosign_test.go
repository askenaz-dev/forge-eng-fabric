package cosign

import (
	"context"
	"regexp"
	"testing"
)

func TestVerifyValidSignatureAccepted(t *testing.T) {
	ids := NewWorkspaceIdentities()
	ids.Configure("ws-1", IdentityRule{
		IssuerExact:    "https://token.actions.githubusercontent.com",
		IdentityRegexp: regexp.MustCompile(`^https://github.com/forge-org/.+@refs/heads/main$`),
	})
	v := NewVerifier(ids, StaticDigestLookup{"app-foo:abc123": "sha256:111"})
	res := v.Combined(context.Background(), "ws-1", ImageMetadata{
		Image: "app-foo:abc123", Digest: "sha256:111",
		Signed:               true,
		OIDCIssuer:           "https://token.actions.githubusercontent.com",
		OIDCIdentity:         "https://github.com/forge-org/app-foo@refs/heads/main",
		HasRekorEntry:        true,
		RekorLogIndex:        42,
		AttestationType:      "slsaprovenance",
		AttestationValid:     true,
	})
	if res.Outcome != OutcomeSuccess {
		t.Fatalf("expected success, got %+v", res)
	}
}

func TestVerifyIdentityMismatchBlocks(t *testing.T) {
	ids := NewWorkspaceIdentities()
	ids.Configure("ws-1", IdentityRule{
		IssuerExact:    "https://token.actions.githubusercontent.com",
		IdentityRegexp: regexp.MustCompile(`^https://github.com/forge-org/.+@refs/heads/main$`),
	})
	v := NewVerifier(ids, StaticDigestLookup{"x:y": "sha256:1"})
	res := v.Verify(context.Background(), "ws-1", ImageMetadata{
		Image: "x:y", Digest: "sha256:1", Signed: true,
		OIDCIssuer: "https://token.actions.githubusercontent.com",
		OIDCIdentity: "https://github.com/attacker/repo@refs/heads/main",
		HasRekorEntry: true,
	})
	if res.Outcome != OutcomeFailed || res.Reason != ReasonIdentityMismatch {
		t.Fatalf("expected identity_mismatch, got %+v", res)
	}
}

func TestVerifyMissingRekorBlocks(t *testing.T) {
	v := NewVerifier(NewWorkspaceIdentities(), nil)
	res := v.Verify(context.Background(), "ws-1", ImageMetadata{Signed: true, HasRekorEntry: false})
	if res.Outcome != OutcomeFailed || res.Reason != ReasonRekorMissing {
		t.Fatalf("expected rekor_entry_missing, got %+v", res)
	}
}

func TestVerifyAttestationDigestMismatch(t *testing.T) {
	v := NewVerifier(nil, StaticDigestLookup{"app:1": "sha256:111"})
	res := v.VerifyAttestation(context.Background(), ImageMetadata{
		Image: "app:1", Digest: "sha256:222",
		AttestationType: "slsaprovenance", AttestationValid: true,
	})
	if res.Outcome != OutcomeFailed || res.Reason != ReasonDigestMismatch {
		t.Fatalf("expected digest_mismatch, got %+v", res)
	}
}

func TestVerifyUnsignedBlocks(t *testing.T) {
	v := NewVerifier(nil, nil)
	res := v.Verify(context.Background(), "ws-1", ImageMetadata{Signed: false})
	if res.Outcome != OutcomeFailed || res.Reason != ReasonUnsigned {
		t.Fatalf("expected unsigned_image, got %+v", res)
	}
}
