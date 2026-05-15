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

// Identity header set the gateway injects on every outbound A2A request,
// and that the inbound flow sets when forwarding into the internal agent
// runtime so the runtime knows it is receiving a call on behalf of an
// external partner.
const (
	HeaderPrincipal     = "X-Forge-Principal"
	HeaderPrincipalKind = "X-Forge-Principal-Kind" // "user" | "service" | "external_agent"
	HeaderTenant        = "X-Forge-Tenant"
	HeaderWorkspace     = "X-Forge-Workspace"
	HeaderCorrelationID = "X-Forge-Correlation-Id"
	HeaderTimestamp     = "X-Forge-Identity-Ts"
	HeaderSignature     = "X-Forge-Identity-Signature"
	HeaderKeyID         = "X-Forge-Identity-Kid"
	HeaderPartnerAuth   = "X-Forge-Partner-Auth"   // inbound auth carrier
)

type signingKey struct {
	ID       string
	Public   ed25519.PublicKey
	Private  ed25519.PrivateKey
	NotAfter time.Time
}

type KeyManager struct {
	mu       sync.RWMutex
	current  *signingKey
	previous *signingKey
	rotation time.Duration
	rng      func(io.Reader) (ed25519.PublicKey, ed25519.PrivateKey, error)
	clock    func() time.Time
}

func NewKeyManager(rotation time.Duration) (*KeyManager, error) {
	km := &KeyManager{rotation: rotation, rng: ed25519.GenerateKey, clock: time.Now}
	if err := km.Rotate(); err != nil {
		return nil, err
	}
	return km, nil
}

func (k *KeyManager) Rotate() error {
	pub, priv, err := k.rng(rand.Reader)
	if err != nil {
		return err
	}
	now := k.clock()
	next := &signingKey{
		ID: fmt.Sprintf("k-%d", now.UnixNano()), Public: pub, Private: priv,
		NotAfter: now.Add(2 * k.rotation),
	}
	k.mu.Lock()
	k.previous = k.current
	k.current = next
	k.mu.Unlock()
	return nil
}

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
			_ = k.Rotate()
		}
	}
}

type IdentityPayload struct {
	Principal     string `json:"principal"`
	PrincipalKind string `json:"principal_kind"`
	Tenant        string `json:"tenant"`
	Workspace     string `json:"workspace"`
	CorrelationID string `json:"correlation_id"`
	Timestamp     int64  `json:"ts"`
}

func (p IdentityPayload) CanonicalBytes() []byte {
	enc := fmt.Sprintf(
		`{"principal":%q,"principal_kind":%q,"tenant":%q,"workspace":%q,"correlation_id":%q,"ts":%d}`,
		p.Principal, p.PrincipalKind, p.Tenant, p.Workspace, p.CorrelationID, p.Timestamp,
	)
	return []byte(enc)
}

func (k *KeyManager) Sign(p IdentityPayload) (sigB64, kid string, ts int64, err error) {
	k.mu.RLock()
	cur := k.current
	k.mu.RUnlock()
	if cur == nil {
		return "", "", 0, errors.New("identity: no current key")
	}
	if p.Timestamp == 0 {
		p.Timestamp = k.clock().Unix()
	}
	sig := ed25519.Sign(cur.Private, p.CanonicalBytes())
	return base64.RawStdEncoding.EncodeToString(sig), cur.ID, p.Timestamp, nil
}

type JWKSResponse struct {
	Keys []JWKSKey `json:"keys"`
}

type JWKSKey struct {
	KeyID    string `json:"kid"`
	Kty      string `json:"kty"`
	Alg      string `json:"alg"`
	Crv      string `json:"crv"`
	X        string `json:"x"`
	Use      string `json:"use"`
	NotAfter int64  `json:"not_after"`
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

func (s *server) jwksHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(s.km.JWKS())
}

func applyIdentityHeaders(req *http.Request, p IdentityPayload, sigB64, kid string) {
	req.Header.Set(HeaderPrincipal, p.Principal)
	req.Header.Set(HeaderPrincipalKind, p.PrincipalKind)
	req.Header.Set(HeaderTenant, p.Tenant)
	req.Header.Set(HeaderWorkspace, p.Workspace)
	req.Header.Set(HeaderCorrelationID, p.CorrelationID)
	req.Header.Set(HeaderTimestamp, fmt.Sprintf("%d", p.Timestamp))
	req.Header.Set(HeaderSignature, sigB64)
	req.Header.Set(HeaderKeyID, kid)
}

func stripBearerScheme(h string) string {
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return h
}
