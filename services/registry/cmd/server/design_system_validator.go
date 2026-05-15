package main

import (
	"fmt"
	"regexp"
	"strings"
)

// SanityValidatorInput carries the materialised tokens.css body, the tenant's
// approved-URL allow-list and the asset's tenant id. The registry resolves
// these and passes them in; the validator is otherwise stateless and safe to
// unit-test against arbitrary stylesheets.
type SanityValidatorInput struct {
	TokensCSS         string
	TenantApprovedURLDomains []string
}

// SanityValidatorVerdict carries the first rule that fired. A passing token
// sheet returns nil.
type SanityValidatorVerdict struct {
	Code    string
	Message string
}

// Layout namespaces (per design Decision 4 and the spec): these custom-property
// prefixes are reserved for the App's base Design System and MUST NOT appear
// in tenant-published token sheets — they would otherwise produce visually
// broken UIs when used as overrides.
var layoutTokenPrefixes = []string{"--space-", "--grid-", "--breakpoint-"}

var urlRefPattern = regexp.MustCompile(`url\(\s*['"]?([^'")]+)['"]?\s*\)`)
var expressionPattern = regexp.MustCompile(`(?i)expression\s*\(`)
var jsSchemePattern = regexp.MustCompile(`(?i)javascript\s*:`)
var customPropertyDeclPattern = regexp.MustCompile(`(--[a-z0-9_-]+)\s*:`)

// runSanityValidator scans `tokens.css` for the rules described in the
// `design-system-catalog` spec: off-tenant URL references, CSS `expression()`
// tokens, JavaScript-scheme values and layout-namespace collisions. The first
// hit short-circuits the scan. Built-in templates bypass this gate; callers
// MUST check `built_in_template` before invoking.
func runSanityValidator(in SanityValidatorInput) *SanityValidatorVerdict {
	if strings.TrimSpace(in.TokensCSS) == "" {
		return &SanityValidatorVerdict{Code: "design_system_tokens_empty", Message: "tokens.css body is empty"}
	}
	if loc := expressionPattern.FindStringIndex(in.TokensCSS); loc != nil {
		return &SanityValidatorVerdict{Code: "design_system_expression_forbidden", Message: "tokens.css uses CSS expression()"}
	}
	if loc := jsSchemePattern.FindStringIndex(in.TokensCSS); loc != nil {
		return &SanityValidatorVerdict{Code: "design_system_js_scheme_forbidden", Message: "tokens.css contains a javascript: URI scheme"}
	}
	for _, match := range urlRefPattern.FindAllStringSubmatch(in.TokensCSS, -1) {
		raw := strings.TrimSpace(match[1])
		// Relative or same-origin references are allowed; only absolute http(s) URLs
		// pointing off-tenant are gated. A `data:` or relative path passes through.
		if strings.HasPrefix(raw, "http://") {
			return &SanityValidatorVerdict{Code: "design_system_url_not_https", Message: "tokens.css references an http:// URL (must be https)"}
		}
		if !strings.HasPrefix(raw, "https://") {
			continue
		}
		host := hostnameOf(raw)
		if host == "" {
			return &SanityValidatorVerdict{Code: "untrusted_url_in_tokens", Message: "tokens.css URL has no resolvable host: " + raw}
		}
		if !hostAllowed(host, in.TenantApprovedURLDomains) {
			return &SanityValidatorVerdict{Code: "untrusted_url_in_tokens", Message: "tokens.css references off-tenant URL: " + raw}
		}
	}
	for _, match := range customPropertyDeclPattern.FindAllStringSubmatch(in.TokensCSS, -1) {
		name := strings.ToLower(match[1])
		for _, prefix := range layoutTokenPrefixes {
			if strings.HasPrefix(name, prefix) {
				return &SanityValidatorVerdict{Code: "layout_namespace_collision", Message: fmt.Sprintf("tokens.css declares layout token %q (reserved for the App base)", name)}
			}
		}
	}
	return nil
}

func hostnameOf(raw string) string {
	rest := strings.TrimPrefix(raw, "https://")
	rest = strings.TrimPrefix(rest, "http://")
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		rest = rest[i+1:]
	}
	if i := strings.Index(rest, ":"); i >= 0 {
		rest = rest[:i]
	}
	return strings.ToLower(rest)
}

func hostAllowed(host string, allow []string) bool {
	host = strings.ToLower(host)
	for _, d := range allow {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}
