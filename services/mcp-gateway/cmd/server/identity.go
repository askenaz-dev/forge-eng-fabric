package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// IdentityHeaders names the canonical headers the gateway injects on every
// outbound MCP request. Downstream MCPs verify X-Forge-Identity-Signature
// against the JWKS-style endpoint the gateway exposes.
const (
	HeaderPrincipal     = "X-Forge-Principal"
	HeaderTenant        = "X-Forge-Tenant"
	HeaderWorkspace     = "X-Forge-Workspace"
	HeaderCorrelationID = "X-Forge-Correlation-Id"
	HeaderTimestamp     = "X-Forge-Identity-Ts"
	HeaderSignature     = "X-Forge-Identity-Signature"
	HeaderKeyID         = "X-Forge-Identity-Kid"
)

// signingKey is one entry in the rotating key set. The gateway holds the
// private half; downstream MCPs fetch the public half from /jwks.
type signingKey struct {
	ID      string
	Public  ed25519.PublicKey
	Private ed25519.PrivateKey
	NotAfter time.Time
}

// KeyManager owns the rotating Ed25519 signing key. It exposes Sign for the
// invoke handler, JWKS for the public side, and starts a background
// goroutine on Start that rotates the current key on a ticker.
type KeyManager struct {
	mu       sync.RWMutex
	current  *signingKey
	previous *signingKey
	rotation time.Duration
	rng      func(io.Reader) (ed25519.PublicKey, ed25519.PrivateKey, error)
	clock    func() time.Time
}

// NewKeyManager builds a KeyManager with a freshly generated initial key.
// rotation is the cadence between rotations; in production 24h, in tests
// callers usually pass a long value and call Rotate manually.
func NewKeyManager(rotation time.Duration) (*KeyManager, error) {
	km := &KeyManager{
		rotation: rotation,
		rng:      ed25519.GenerateKey,
		clock:    time.Now,
	}
	if err := km.Rotate(); err != nil {
		return nil, err
	}
	return km, nil
}

// Rotate generates a new key and shifts the previous current → previous,
// so signatures issued just before the rotation continue to verify for
// one rotation interval after they were signed. The previous key is
// dropped on the rotation after that.
func (k *KeyManager) Rotate() error {
	pub, priv, err := k.rng(rand.Reader)
	if err != nil {
		return err
	}
	now := k.clock()
	next := &signingKey{
		ID:       fmt.Sprintf("k-%d", now.UnixNano()),
		Public:   pub,
		Private:  priv,
		NotAfter: now.Add(2 * k.rotation), // valid for one extra interval
	}
	k.mu.Lock()
	k.previous = k.current
	k.current = next
	k.mu.Unlock()
	return nil
}

// Start runs Rotate on a ticker until ctx is cancelled. Cancel ctx on
// shutdown to drain the goroutine.
func (k *KeyManager) Start(ctx context.Context) {
	if k.rotation <= 0 {
		return
	}
	t := time.NewTicker(k.rotation)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := k.Rotate(); err != nil {
				// Sticky failure: keep the previous key alive. Real
				// production wires this to an alert.
				continue
			}
		}
	}
}

// IdentityPayload is the canonical struct over which the signature is
// computed. The byte form is the deterministic JSON encoding of these
// fields — downstream MCPs MUST encode the headers into the same byte
// shape before verifying.
type IdentityPayload struct {
	Principal     string `json:"principal"`
	Tenant        string `json:"tenant"`
	Workspace     string `json:"workspace"`
	CorrelationID string `json:"correlation_id"`
	Timestamp     int64  `json:"ts"`
}

// CanonicalBytes returns the deterministic byte form. Field order is
// fixed; whitespace is canonicalized by encoding/json.
func (p IdentityPayload) CanonicalBytes() []byte {
	// Build the canonical form manually to avoid map-ordering issues.
	enc := fmt.Sprintf(
		`{"principal":%q,"tenant":%q,"workspace":%q,"correlation_id":%q,"ts":%d}`,
		p.Principal, p.Tenant, p.Workspace, p.CorrelationID, p.Timestamp,
	)
	return []byte(enc)
}

// Sign returns the signature, key id and timestamp the gateway injects
// into the outbound request's headers.
func (k *KeyManager) Sign(p IdentityPayload) (signatureB64, kid string, ts int64, err error) {
	k.mu.RLock()
	cur := k.current
	k.mu.RUnlock()
	if cur == nil {
		return "", "", 0, errors.New("identity: no current key; Rotate has not been called")
	}
	if p.Timestamp == 0 {
		p.Timestamp = k.clock().Unix()
	}
	sig := ed25519.Sign(cur.Private, p.CanonicalBytes())
	return base64.RawStdEncoding.EncodeToString(sig), cur.ID, p.Timestamp, nil
}

// JWKSResponse is the over-the-wire shape served by /jwks. Downstream
// MCPs fetch this to obtain the current and previous public keys.
type JWKSResponse struct {
	Keys []JWKSKey `json:"keys"`
}

type JWKSKey struct {
	KeyID     string `json:"kid"`
	Kty       string `json:"kty"`
	Alg       string `json:"alg"`
	Crv       string `json:"crv"`
	X         string `json:"x"` // base64url(public key bytes)
	Use       string `json:"use"`
	NotAfter  int64  `json:"not_after"`
}

func (k *KeyManager) JWKS() JWKSResponse {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := JWKSResponse{}
	for _, sk := range []*signingKey{k.current, k.previous} {
		if sk == nil {
			continue
		}
		out.Keys = append(out.Keys, JWKSKey{
			KeyID:    sk.ID,
			Kty:      "OKP",
			Alg:      "EdDSA",
			Crv:      "Ed25519",
			X:        base64.RawURLEncoding.EncodeToString(sk.Public),
			Use:      "sig",
			NotAfter: sk.NotAfter.Unix(),
		})
	}
	return out
}

// jwksHandler returns the JWKS response so downstream MCPs can verify
// our signatures. No auth; the public keys are not secrets.
func (s *server) jwksHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(s.km.JWKS())
}

// Verify checks a signature against the current or previous key. Useful
// for unit tests and for any in-process consumer (e.g. integration tests).
func (k *KeyManager) Verify(p IdentityPayload, signatureB64, kid string) bool {
	sig, err := base64.RawStdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false
	}
	k.mu.RLock()
	defer k.mu.RUnlock()
	for _, sk := range []*signingKey{k.current, k.previous} {
		if sk == nil || sk.ID != kid {
			continue
		}
		if ed25519.Verify(sk.Public, p.CanonicalBytes(), sig) {
			return true
		}
	}
	return false
}

// applyIdentityHeaders sets the canonical identity header set on req in
// the order downstream MCPs expect them in.
func applyIdentityHeaders(req *http.Request, p IdentityPayload, signatureB64, kid string) {
	req.Header.Set(HeaderPrincipal, p.Principal)
	req.Header.Set(HeaderTenant, p.Tenant)
	req.Header.Set(HeaderWorkspace, p.Workspace)
	req.Header.Set(HeaderCorrelationID, p.CorrelationID)
	req.Header.Set(HeaderTimestamp, fmt.Sprintf("%d", p.Timestamp))
	req.Header.Set(HeaderSignature, signatureB64)
	req.Header.Set(HeaderKeyID, kid)
}

// stripBearerScheme removes the `Bearer ` prefix from an Authorization
// header value. Used in tests to compare claims surfaces.
func stripBearerScheme(h string) string {
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return h
}
