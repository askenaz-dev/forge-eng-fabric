package main

import "testing"

func TestSanityValidator_PassingTokens(t *testing.T) {
	v := runSanityValidator(SanityValidatorInput{
		TokensCSS: `
:root {
  --color-primary: #FF6B35;
  --color-surface: #FFFFFF;
  --font-sans: "Inter", system-ui;
  --radius-2: 8px;
}
`,
		TenantApprovedURLDomains: []string{"acme.example.com"},
	})
	if v != nil {
		t.Fatalf("expected nil verdict, got code=%s message=%s", v.Code, v.Message)
	}
}

func TestSanityValidator_RejectsOffTenantURL(t *testing.T) {
	v := runSanityValidator(SanityValidatorInput{
		TokensCSS: `:root { --logo-bg: url("https://attacker.example/x.png"); }`,
		TenantApprovedURLDomains: []string{"acme.example.com"},
	})
	if v == nil || v.Code != "untrusted_url_in_tokens" {
		t.Fatalf("expected untrusted_url_in_tokens, got %+v", v)
	}
}

func TestSanityValidator_AllowsApprovedHost(t *testing.T) {
	v := runSanityValidator(SanityValidatorInput{
		TokensCSS: `:root { --logo-bg: url("https://cdn.acme.example.com/logo.svg"); }`,
		TenantApprovedURLDomains: []string{"acme.example.com"},
	})
	if v != nil {
		t.Fatalf("expected nil verdict for approved host, got %+v", v)
	}
}

func TestSanityValidator_RejectsJavaScriptScheme(t *testing.T) {
	v := runSanityValidator(SanityValidatorInput{
		TokensCSS: `:root { --x: javascript:alert(1); }`,
	})
	if v == nil || v.Code != "design_system_js_scheme_forbidden" {
		t.Fatalf("expected design_system_js_scheme_forbidden, got %+v", v)
	}
}

func TestSanityValidator_RejectsLayoutCollision(t *testing.T) {
	v := runSanityValidator(SanityValidatorInput{
		TokensCSS: `:root { --space-1: 4px; --color-primary: red; }`,
	})
	if v == nil || v.Code != "layout_namespace_collision" {
		t.Fatalf("expected layout_namespace_collision, got %+v", v)
	}
}

func TestSanityValidator_RejectsExpression(t *testing.T) {
	v := runSanityValidator(SanityValidatorInput{
		TokensCSS: `:root { --x: expression(alert(1)); }`,
	})
	if v == nil || v.Code != "design_system_expression_forbidden" {
		t.Fatalf("expected design_system_expression_forbidden, got %+v", v)
	}
}

func TestSanityValidator_RejectsHTTPScheme(t *testing.T) {
	v := runSanityValidator(SanityValidatorInput{
		TokensCSS:                `:root { --logo: url("http://acme.example.com/logo.svg"); }`,
		TenantApprovedURLDomains: []string{"acme.example.com"},
	})
	if v == nil || v.Code != "design_system_url_not_https" {
		t.Fatalf("expected design_system_url_not_https, got %+v", v)
	}
}

func TestSanityValidator_EmptyTokensRejected(t *testing.T) {
	v := runSanityValidator(SanityValidatorInput{TokensCSS: ""})
	if v == nil || v.Code != "design_system_tokens_empty" {
		t.Fatalf("expected design_system_tokens_empty, got %+v", v)
	}
}

func TestDesignSystemManifest_RejectsMissingScreenshots(t *testing.T) {
	raw := map[string]any{
		"tokens":            "https://platform.forge.example/ds/tokens.css",
		"tokens_sha256":     "0000000000000000000000000000000000000000000000000000000000000000",
		"components":        "https://platform.forge.example/ds/components.tgz",
		"components_sha256": "0000000000000000000000000000000000000000000000000000000000000000",
		"fonts":             []any{map[string]any{"family": "Inter", "weights": []any{400}, "source": "https://platform.forge.example/ds/inter.woff2"}},
		"screenshots": map[string]any{
			"light":        "https://platform.forge.example/ds/light.png",
			"light_sha256": "0000000000000000000000000000000000000000000000000000000000000000",
		},
		"use_case": "Minimal",
	}
	_, fault := validateDesignSystemManifest(raw)
	if fault == nil || fault.Code != "missing_design_system_manifest_field" {
		t.Fatalf("expected missing_design_system_manifest_field for missing screenshots.dark, got %+v", fault)
	}
}

func TestDesignSystemManifest_RejectsMissingDigest(t *testing.T) {
	raw := map[string]any{
		"tokens":            "https://platform.forge.example/ds/tokens.css",
		"components":        "https://platform.forge.example/ds/components.tgz",
		"components_sha256": "0000000000000000000000000000000000000000000000000000000000000000",
		"fonts":             []any{map[string]any{"family": "Inter", "weights": []any{400}, "source": "https://platform.forge.example/ds/inter.woff2"}},
		"screenshots": map[string]any{
			"light":        "https://platform.forge.example/ds/light.png",
			"light_sha256": "0000000000000000000000000000000000000000000000000000000000000000",
			"dark":         "https://platform.forge.example/ds/dark.png",
			"dark_sha256":  "0000000000000000000000000000000000000000000000000000000000000000",
		},
		"use_case": "Minimal",
	}
	_, fault := validateDesignSystemManifest(raw)
	if fault == nil || fault.Code != "missing_token_digest" {
		t.Fatalf("expected missing_token_digest for missing tokens_sha256, got %+v", fault)
	}
}

func TestDesignSystemEvalThresholds(t *testing.T) {
	failing := failingDesignSystemEvalScores(map[string]any{
		"accessibility":  0.85,
		"brand_fidelity": 0.9,
	})
	if _, ok := failing["accessibility"]; !ok {
		t.Fatalf("expected accessibility to be flagged below 0.9 threshold, got %+v", failing)
	}
	if _, ok := failing["brand_fidelity"]; ok {
		t.Fatalf("expected brand_fidelity to pass at 0.9, got %+v", failing)
	}
}
