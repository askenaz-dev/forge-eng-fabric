package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Partner is one enrollment record. The credential is a high-entropy
// shared secret that the partner uses to sign inbound requests (HMAC).
// Production deployments terminate mTLS at an envoy/ingress in front of
// the gateway and pass the verified partner identity as a header; this
// in-process variant covers the test / dev paths and any deployment that
// has not yet wired the mTLS edge.
type Partner struct {
	Name            string    `json:"name"`
	TenantID        string    `json:"tenant_id"`
	WorkspaceID     string    `json:"workspace_id,omitempty"`
	AllowedAssets   []string  `json:"allowed_assets"`
	CredentialB64   string    `json:"credential_b64"`
	CreatedAt       time.Time `json:"created_at"`
	CreatedBy       string    `json:"created_by,omitempty"`
	credentialBytes []byte    // decoded, kept off-wire
}

// PartnerStore is the in-memory enrollment registry. Production hosts a
// DB-backed implementation against the artifact_store_binding-style
// shape; this in-memory version is enough for §5 scope and exercises the
// same external API.
type PartnerStore struct {
	mu    sync.RWMutex
	byKey map[string]*Partner // key: tenant_id/name
}

func NewPartnerStore() *PartnerStore {
	return &PartnerStore{byKey: map[string]*Partner{}}
}

func partnerKey(tenant, name string) string { return tenant + "/" + name }

func (s *PartnerStore) Enroll(p Partner) (Partner, error) {
	if p.Name == "" || p.TenantID == "" || p.CredentialB64 == "" {
		return Partner{}, errors.New("name, tenant_id and credential_b64 are required")
	}
	raw, err := base64.StdEncoding.DecodeString(p.CredentialB64)
	if err != nil {
		return Partner{}, errors.New("credential_b64: must be base64")
	}
	if len(raw) < 16 {
		return Partner{}, errors.New("credential must decode to >= 16 bytes")
	}
	p.credentialBytes = raw
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	s.byKey[partnerKey(p.TenantID, p.Name)] = &p
	s.mu.Unlock()
	return p, nil
}

func (s *PartnerStore) Lookup(tenant, name string) (Partner, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.byKey[partnerKey(tenant, name)]
	if !ok {
		return Partner{}, false
	}
	return *p, true
}

func (s *PartnerStore) List(tenant string) []Partner {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Partner
	for _, p := range s.byKey {
		if p.TenantID == tenant {
			cp := *p
			cp.credentialBytes = nil
			out = append(out, cp)
		}
	}
	return out
}

// Authenticate is the inbound auth check. The partner sends:
//
//	X-Forge-Partner-Auth: <partner-name>;<base64-hmac-of-body>
//
// The gateway looks up the partner, computes HMAC-SHA256(body) under the
// stored credential and constant-time compares. On success it returns the
// resolved partner record so the handler can apply allowlist + policy.
func (s *PartnerStore) Authenticate(tenant string, header string, body []byte) (Partner, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return Partner{}, errors.New("missing X-Forge-Partner-Auth")
	}
	idx := strings.IndexByte(header, ';')
	if idx <= 0 || idx == len(header)-1 {
		return Partner{}, errors.New("malformed partner auth (expect <name>;<sig>)")
	}
	name := strings.TrimSpace(header[:idx])
	givenB64 := strings.TrimSpace(header[idx+1:])
	given, err := base64.StdEncoding.DecodeString(givenB64)
	if err != nil {
		return Partner{}, errors.New("partner auth signature not base64")
	}
	p, ok := s.Lookup(tenant, name)
	if !ok {
		return Partner{}, errors.New("unknown_partner")
	}
	mac := hmac.New(sha256.New, p.credentialBytes)
	_, _ = mac.Write(body)
	expected := mac.Sum(nil)
	if subtle.ConstantTimeCompare(given, expected) != 1 {
		return Partner{}, errors.New("partner auth signature invalid")
	}
	return p, nil
}

// AllowsAsset reports whether the partner is allowed to invoke the given
// asset. Empty AllowedAssets means deny-all (matches the secure default
// in the migration spec for external integrations).
func (p Partner) AllowsAsset(assetID string) bool {
	for _, a := range p.AllowedAssets {
		if a == assetID {
			return true
		}
	}
	return false
}

// partnersHandler implements POST /v1/gw/a2a/partners and GET /v1/gw/a2a/partners.
// The Tenant of the caller scopes the enrollment surface.
func (s *server) partnersHandler(w http.ResponseWriter, r *http.Request) {
	tenant, _ := r.Context().Value(ctxKeyTenant).(string)
	if tenant == "" {
		writeJSONErr(w, 401, "missing_identity", "tenant must be set")
		return
	}
	if r.Method == http.MethodGet {
		out := s.partners.List(tenant)
		// Strip the credential before responding — the store doesn't
		// even include it in the projection but defense-in-depth.
		for i := range out {
			out[i].CredentialB64 = ""
		}
		w.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": out})
		return
	}
	var req Partner
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErr(w, 400, "invalid_body", err.Error())
		return
	}
	req.TenantID = tenant
	req.CreatedBy, _ = r.Context().Value(ctxKeyPrincipal).(string)
	p, err := s.partners.Enroll(req)
	if err != nil {
		writeJSONErr(w, 400, "invalid_enrollment", err.Error())
		return
	}
	// Echo the partner record without the credential — operators
	// retrieve the credential out-of-band (vault, signed envelope) and
	// hand it to the partner separately.
	p.CredentialB64 = ""
	writeJSON(w, 201, p)
}
