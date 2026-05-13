// Package auth implements personal access tokens, OIDC device-code login and
// scope checks for the developer skill gateway.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

// Scope enumerates the closed set of PAT permissions accepted by the gateway.
type Scope string

const (
	ScopeRead    Scope = "gateway.read"
	ScopeInstall Scope = "gateway.install"
	ScopeInvoke  Scope = "gateway.invoke"
)

var allowedScopes = map[Scope]struct{}{ScopeRead: {}, ScopeInstall: {}, ScopeInvoke: {}}

// MaxLifetime is the hardest cap on PAT expiry (90 days).
const MaxLifetime = 90 * 24 * time.Hour

// PAT is a personal access token row.
type PAT struct {
	ID                 uuid.UUID  `json:"id"`
	TenantID           uuid.UUID  `json:"tenant_id"`
	DeveloperSub       string     `json:"developer_sub"`
	AssumeWorkspaceID  uuid.UUID  `json:"assume_workspace_id"`
	Scopes             []Scope    `json:"scopes"`
	AssetAllowlist     []string   `json:"asset_allowlist"`
	CreatedBy          string     `json:"created_by"`
	CreatedAt          time.Time  `json:"created_at"`
	ExpiresAt          time.Time  `json:"expires_at"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
	RevokedAt          *time.Time `json:"revoked_at,omitempty"`
}

// Issued is what Issue returns: the database row plus the plaintext token,
// shown to the caller once and never persisted in plain.
type Issued struct {
	PAT       PAT    `json:"pat"`
	Plaintext string `json:"plaintext"`
}

// IssueRequest is the inbound payload for POST /v1/gateway/tokens.
type IssueRequest struct {
	TenantID          uuid.UUID `json:"tenant_id"`
	DeveloperSub      string    `json:"developer_sub"`
	AssumeWorkspaceID uuid.UUID `json:"assume_workspace_id"`
	Scopes            []Scope   `json:"scopes"`
	AssetAllowlist    []string  `json:"asset_allowlist"`
	CreatedBy         string    `json:"created_by"`
	Lifetime          time.Duration `json:"lifetime"`
}

// Service issues, looks up and revokes PATs against the gateway_token table.
// Argon2id is used to hash the plaintext token so the database leak does not
// expose live credentials.
type Service struct {
	Pool *pgxpool.Pool
	Now  func() time.Time
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{Pool: pool, Now: func() time.Time { return time.Now().UTC() }}
}

// Issue mints a new PAT. The plaintext is shown to the caller exactly once.
func (s *Service) Issue(ctx context.Context, req IssueRequest) (*Issued, error) {
	if req.TenantID == uuid.Nil {
		return nil, errors.New("tenant_id is required")
	}
	if req.AssumeWorkspaceID == uuid.Nil {
		return nil, errors.New("assume_workspace_id is required")
	}
	if req.DeveloperSub == "" {
		return nil, errors.New("developer_sub is required")
	}
	if len(req.Scopes) == 0 {
		return nil, errors.New("at least one scope is required")
	}
	for _, sc := range req.Scopes {
		if _, ok := allowedScopes[sc]; !ok {
			return nil, fmt.Errorf("invalid_scope: %s", sc)
		}
	}
	lifetime := req.Lifetime
	if lifetime <= 0 {
		lifetime = MaxLifetime
	}
	if lifetime > MaxLifetime {
		return nil, fmt.Errorf("lifetime exceeds 90d maximum")
	}

	plaintext, err := mintTokenString()
	if err != nil {
		return nil, err
	}
	hashed := argon2idHash(plaintext)

	now := s.Now()
	scopesText := make([]string, len(req.Scopes))
	for i, sc := range req.Scopes {
		scopesText[i] = string(sc)
	}
	var pat PAT
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO gateway_token(tenant_id,developer_sub,assume_workspace_id,scopes,asset_allowlist,hashed_secret,created_by,created_at,expires_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id,tenant_id,developer_sub,assume_workspace_id,scopes,asset_allowlist,created_by,created_at,expires_at,last_used_at,revoked_at`,
		req.TenantID, req.DeveloperSub, req.AssumeWorkspaceID, scopesText, req.AssetAllowlist, hashed, req.CreatedBy, now, now.Add(lifetime)).
		Scan(&pat.ID, &pat.TenantID, &pat.DeveloperSub, &pat.AssumeWorkspaceID, &scopesText, &pat.AssetAllowlist, &pat.CreatedBy, &pat.CreatedAt, &pat.ExpiresAt, &pat.LastUsedAt, &pat.RevokedAt)
	if err != nil {
		return nil, err
	}
	pat.Scopes = parseScopes(scopesText)
	return &Issued{PAT: pat, Plaintext: plaintext}, nil
}

// Lookup validates a presented PAT plaintext and returns the row. Returns
// ErrUnknown if no row matches, ErrExpired / ErrRevoked otherwise.
func (s *Service) Lookup(ctx context.Context, plaintext string) (*PAT, error) {
	if plaintext == "" {
		return nil, ErrUnknown
	}
	hashed := argon2idHash(plaintext)
	row := s.Pool.QueryRow(ctx,
		`SELECT id,tenant_id,developer_sub,assume_workspace_id,scopes,asset_allowlist,created_by,created_at,expires_at,last_used_at,revoked_at
		 FROM gateway_token WHERE hashed_secret=$1`, hashed)
	var pat PAT
	var scopesText []string
	if err := row.Scan(&pat.ID, &pat.TenantID, &pat.DeveloperSub, &pat.AssumeWorkspaceID, &scopesText, &pat.AssetAllowlist, &pat.CreatedBy, &pat.CreatedAt, &pat.ExpiresAt, &pat.LastUsedAt, &pat.RevokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnknown
		}
		return nil, err
	}
	pat.Scopes = parseScopes(scopesText)
	now := s.Now()
	if pat.RevokedAt != nil && !pat.RevokedAt.After(now) {
		return nil, ErrRevoked
	}
	if !pat.ExpiresAt.After(now) {
		return nil, ErrExpired
	}
	_, _ = s.Pool.Exec(ctx, `UPDATE gateway_token SET last_used_at=$2 WHERE id=$1`, pat.ID, now)
	return &pat, nil
}

// Revoke marks a PAT as revoked. Returns ErrUnknown if the PAT does not exist.
func (s *Service) Revoke(ctx context.Context, id uuid.UUID, actor string) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE gateway_token SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`,
		id, s.Now())
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUnknown
	}
	return nil
}

// HasScope returns true when the PAT carries the requested scope.
func (p *PAT) HasScope(s Scope) bool {
	for _, x := range p.Scopes {
		if x == s {
			return true
		}
	}
	return false
}

// AllowsAsset returns true when the PAT has no allowlist (== all) or the
// requested asset id is on the allowlist.
func (p *PAT) AllowsAsset(assetID string) bool {
	if len(p.AssetAllowlist) == 0 {
		return true
	}
	for _, x := range p.AssetAllowlist {
		if x == assetID {
			return true
		}
	}
	return false
}

var (
	ErrUnknown = errors.New("token_unknown")
	ErrExpired = errors.New("token_expired")
	ErrRevoked = errors.New("token_revoked")
)

func mintTokenString() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "forge_pat_" + hex.EncodeToString(buf), nil
}

// argon2idHash uses a fixed pepper'd salt derived from the token itself. We
// purposely do NOT use a per-row random salt — the table is indexed by the
// hash, so the lookup needs to be deterministic for a given plaintext. The
// argon2id parameters still make brute-forcing the table impractical.
func argon2idHash(plaintext string) string {
	salt := []byte("forge-skill-gateway-v1")
	sum := argon2.IDKey([]byte(plaintext), salt, 2, 64*1024, 2, 32)
	return hex.EncodeToString(sum)
}

// ConstantTimeEqual is exposed so callers can compare strings safely without
// importing crypto/subtle directly.
func ConstantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func parseScopes(text []string) []Scope {
	out := make([]Scope, 0, len(text))
	for _, s := range text {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, Scope(s))
	}
	return out
}
