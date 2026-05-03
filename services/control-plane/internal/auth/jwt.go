package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type ctxKey int

const principalKey ctxKey = 1

// Principal is the authenticated user extracted from the JWT.
type Principal struct {
	Subject  string
	Username string
	Email    string
	Roles    []string
	Groups   []string
	Raw      map[string]any
}

// Verifier validates Keycloak-issued JWTs.
type Verifier struct {
	issuer   string
	audience string
	keys     keyfunc.Keyfunc
}

// NewKeycloakVerifier fetches the JWKS from {issuer}/protocol/openid-connect/certs.
func NewKeycloakVerifier(issuer, audience string) (*Verifier, error) {
	jwksURL := strings.TrimRight(issuer, "/") + "/protocol/openid-connect/certs"
	k, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("jwks: %w", err)
	}
	return &Verifier{issuer: issuer, audience: audience, keys: k}, nil
}

func (v *Verifier) verify(tokenStr string) (*Principal, error) {
	tok, err := jwt.Parse(tokenStr, v.keys.Keyfunc, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	if iss, _ := claims["iss"].(string); iss != v.issuer {
		return nil, fmt.Errorf("unexpected issuer %q", iss)
	}
	// Audience may be a string or array. Keycloak sets `aud` to the client we mapped.
	if !audienceMatches(claims["aud"], v.audience) {
		return nil, fmt.Errorf("audience missing %q", v.audience)
	}
	p := &Principal{Raw: claims}
	if s, _ := claims["sub"].(string); s != "" {
		p.Subject = s
	}
	if u, _ := claims["preferred_username"].(string); u != "" {
		p.Username = u
	}
	if e, _ := claims["email"].(string); e != "" {
		p.Email = e
	}
	if g, ok := claims["groups"].([]any); ok {
		for _, x := range g {
			if s, ok := x.(string); ok {
				p.Groups = append(p.Groups, s)
			}
		}
	}
	if ra, ok := claims["realm_access"].(map[string]any); ok {
		if rs, ok := ra["roles"].([]any); ok {
			for _, x := range rs {
				if s, ok := x.(string); ok {
					p.Roles = append(p.Roles, s)
				}
			}
		}
	}
	return p, nil
}

func audienceMatches(aud any, want string) bool {
	switch v := aud.(type) {
	case string:
		return v == want
	case []any:
		for _, x := range v {
			if s, _ := x.(string); s == want {
				return true
			}
		}
	}
	return false
}

// Middleware returns an HTTP middleware that requires a valid Bearer JWT.
func Middleware(v *Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("authorization")
			if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			tok := strings.TrimSpace(h[7:])
			p, err := v.verify(tok)
			if err != nil {
				http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), principalKey, p)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext returns the principal previously stored by Middleware.
func FromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey).(*Principal)
	return p, ok
}
