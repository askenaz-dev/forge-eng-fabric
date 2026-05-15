package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

// DesignSystemManifest is the canonical wire shape of a `design_system` asset's
// `manifest` block. See openspec/changes/design-system-catalog/specs/ai-asset-registry/spec.md
// for the contract: every field is required at publication time, every URL
// MUST be sha256-pinned, and the `use_case` copy is bounded at 240 characters.
type DesignSystemManifest struct {
	Tokens          string                  `json:"tokens"`
	TokensSHA256    string                  `json:"tokens_sha256"`
	Components      string                  `json:"components"`
	ComponentsSHA256 string                 `json:"components_sha256"`
	Fonts           []DesignSystemFont      `json:"fonts"`
	Screenshots     DesignSystemScreenshots `json:"screenshots"`
	UseCase         string                  `json:"use_case"`
}

// DesignSystemFont is one entry in `manifest.fonts`. `source` is the absolute
// HTTPS URL the runtime preloads at App boot.
type DesignSystemFont struct {
	Family  string `json:"family"`
	Weights []int  `json:"weights"`
	Italic  bool   `json:"italic"`
	Source  string `json:"source"`
}

// DesignSystemScreenshots holds the light/dark hero images displayed in the
// wizard catalog and the App Settings swap UI.
type DesignSystemScreenshots struct {
	Light       string `json:"light"`
	LightSHA256 string `json:"light_sha256"`
	Dark        string `json:"dark"`
	DarkSHA256  string `json:"dark_sha256"`
}

// designSystemUseCaseMaxLen mirrors the spec ("at most 240 characters").
const designSystemUseCaseMaxLen = 240

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// validateDesignSystemManifest enforces the field-presence and shape rules. It
// is invoked at asset creation time (for non-built-in templates) and at lifecycle
// transitions to `approved`. The returned error carries the canonical 4xx code
// the handler must surface in the JSON body so the wizard / CLI can branch on it.
func validateDesignSystemManifest(raw map[string]any) (DesignSystemManifest, *validationFault) {
	if raw == nil {
		return DesignSystemManifest{}, &validationFault{Code: "missing_design_system_manifest", Message: "design_system asset requires manifest block"}
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return DesignSystemManifest{}, &validationFault{Code: "missing_design_system_manifest", Message: "design_system manifest is not encodable: " + err.Error()}
	}
	var manifest DesignSystemManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return DesignSystemManifest{}, &validationFault{Code: "missing_design_system_manifest", Message: "design_system manifest is malformed: " + err.Error()}
	}
	required := []struct {
		field string
		value string
	}{
		{"manifest.tokens", manifest.Tokens},
		{"manifest.components", manifest.Components},
		{"manifest.screenshots.light", manifest.Screenshots.Light},
		{"manifest.screenshots.dark", manifest.Screenshots.Dark},
		{"manifest.use_case", manifest.UseCase},
	}
	for _, f := range required {
		if strings.TrimSpace(f.value) == "" {
			return manifest, &validationFault{Code: "missing_design_system_manifest_field", Message: "missing " + f.field}
		}
	}
	if len(manifest.Fonts) == 0 {
		return manifest, &validationFault{Code: "missing_design_system_manifest_field", Message: "missing manifest.fonts"}
	}
	for i, f := range manifest.Fonts {
		if strings.TrimSpace(f.Family) == "" {
			return manifest, &validationFault{Code: "missing_design_system_manifest_field", Message: fmt.Sprintf("missing manifest.fonts[%d].family", i)}
		}
		if strings.TrimSpace(f.Source) == "" {
			return manifest, &validationFault{Code: "missing_design_system_manifest_field", Message: fmt.Sprintf("missing manifest.fonts[%d].source", i)}
		}
	}
	if len(manifest.UseCase) > designSystemUseCaseMaxLen {
		return manifest, &validationFault{Code: "design_system_use_case_too_long", Message: fmt.Sprintf("manifest.use_case is %d chars (max %d)", len(manifest.UseCase), designSystemUseCaseMaxLen)}
	}
	for _, pair := range []struct {
		urlField  string
		urlValue  string
		hashField string
		hashValue string
	}{
		{"manifest.tokens", manifest.Tokens, "manifest.tokens_sha256", manifest.TokensSHA256},
		{"manifest.components", manifest.Components, "manifest.components_sha256", manifest.ComponentsSHA256},
		{"manifest.screenshots.light", manifest.Screenshots.Light, "manifest.screenshots.light_sha256", manifest.Screenshots.LightSHA256},
		{"manifest.screenshots.dark", manifest.Screenshots.Dark, "manifest.screenshots.dark_sha256", manifest.Screenshots.DarkSHA256},
	} {
		if !strings.HasPrefix(pair.urlValue, "https://") {
			return manifest, &validationFault{Code: "design_system_url_not_https", Message: pair.urlField + " must be HTTPS"}
		}
		if _, err := url.Parse(pair.urlValue); err != nil {
			return manifest, &validationFault{Code: "design_system_url_invalid", Message: pair.urlField + ": " + err.Error()}
		}
		if !sha256Pattern.MatchString(strings.ToLower(pair.hashValue)) {
			return manifest, &validationFault{Code: "missing_token_digest", Message: pair.urlField + " missing or malformed " + pair.hashField}
		}
	}
	return manifest, nil
}

// designSystemEvalThresholds gates the `proposed → approved` transition. They
// supplement the generic per-trust-level quality/safety/cost/latency check.
var designSystemEvalThresholds = map[string]float64{
	"accessibility":  0.9,
	"brand_fidelity": 0.8,
}

// failingDesignSystemEvalScores reports which design-system dimensions fall
// below the published thresholds. Returns an empty map when the asset meets
// every gate. Built-in templates ship with the scores baked into their seed
// data and pass this check by construction.
func failingDesignSystemEvalScores(scores map[string]any) map[string]any {
	failing := map[string]any{}
	for dim, threshold := range designSystemEvalThresholds {
		value, ok := numeric(scores[dim])
		if !ok || value < threshold {
			failing[dim] = scores[dim]
		}
	}
	return failing
}

// transitionDesignSystemToApproved runs the design-system specific approval
// gate: structural manifest, sanity validator pass (with built-in bypass) and
// accessibility/brand_fidelity thresholds. Returns nil to allow the
// transition; the caller proceeds to the generic UPDATE.
func (s *server) transitionDesignSystemToApproved(r *http.Request, id, version string, evalScores map[string]any) *validationFault {
	var (
		manifestBlob []byte
		builtIn      bool
		tenantID     string
	)
	err := s.pool.QueryRow(r.Context(),
		`SELECT COALESCE(design_system_manifest,'null'::jsonb), COALESCE(built_in_template,false), tenant_id
		 FROM asset WHERE id=$1 AND version=$2`,
		id, version,
	).Scan(&manifestBlob, &builtIn, &tenantID)
	if err != nil {
		return &validationFault{Code: "design_system_asset_not_found", Message: err.Error()}
	}
	var manifestMap map[string]any
	if len(manifestBlob) > 0 && string(manifestBlob) != "null" {
		if jerr := json.Unmarshal(manifestBlob, &manifestMap); jerr != nil {
			return &validationFault{Code: "missing_design_system_manifest", Message: "stored manifest is malformed: " + jerr.Error()}
		}
	}
	manifest, mfault := validateDesignSystemManifest(manifestMap)
	if mfault != nil {
		return mfault
	}
	if !builtIn {
		// Fetch the tokens.css body and feed it to the sanity validator.
		// Off-tenant URL allowance is governed by the tenant's
		// `tenant_approved_url_domains` setting; absent the setting only the
		// asset's tenant scope is permitted.
		body, ferr := fetchHTTPBody(manifest.Tokens)
		if ferr != nil {
			return &validationFault{Code: "design_system_tokens_unreachable", Message: ferr.Error()}
		}
		allow := s.tenantApprovedURLDomains(r, tenantID)
		if verdict := runSanityValidator(SanityValidatorInput{TokensCSS: body, TenantApprovedURLDomains: allow}); verdict != nil {
			return &validationFault{Code: verdict.Code, Message: verdict.Message}
		}
	} else {
		s.recordAuditBypass(r, id, version, "built_in_template")
	}
	if failing := failingDesignSystemEvalScores(evalScores); len(failing) > 0 {
		body, _ := json.Marshal(failing)
		return &validationFault{Code: "eval_below_threshold", Message: "design-system eval thresholds not met: " + string(body)}
	}
	return nil
}

// fetchHTTPBody pulls a short HTTPS body for the sanity validator. Limited to
// 1 MiB to keep the worst-case publication cost bounded.
func fetchHTTPBody(rawURL string) (string, error) {
	if _, err := url.Parse(rawURL); err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("%s returned %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// tenantApprovedURLDomains returns the URL allow-list for a tenant. The list
// is populated out-of-band by the platform-team via the control-plane; absent
// configuration, an empty list is returned and only data-URIs / same-origin
// references survive the validator.
func (s *server) tenantApprovedURLDomains(r *http.Request, tenantID string) []string {
	// In Phase 0 the allow-list is the literal env var. A follow-up wires
	// per-tenant configuration through the control plane.
	raw := strings.TrimSpace(os.Getenv("FORGE_DS_ALLOWED_DOMAINS_" + strings.ReplaceAll(tenantID, "-", "_")))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("FORGE_DS_ALLOWED_DOMAINS_DEFAULT"))
	}
	if raw == "" {
		return nil
	}
	out := []string{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *server) recordAuditBypass(r *http.Request, assetID, version, reason string) {
	cid, _ := r.Context().Value(cidKey).(string)
	sub, _ := r.Context().Value(subjectKey).(string)
	envelope := map[string]any{
		"specversion":        "1.0",
		"id":                 uuid.NewString(),
		"source":             "forge://service/registry",
		"type":               "com.forge.asset.design_system.validator_bypassed.v1",
		"subject":            "asset/" + assetID + "@" + version,
		"time":               time.Now().UTC().Format(time.RFC3339Nano),
		"datacontenttype":    "application/json",
		"forgeactor":         "user:" + sub,
		"forgecorrelationid": cid,
		"data": map[string]any{
			"asset_id":           assetID,
			"version":            version,
			"validator_bypassed": true,
			"reason":             reason,
		},
	}
	body, _ := json.Marshal(envelope)
	_ = s.kc.ProduceSync(r.Context(), &kgo.Record{Topic: s.topic, Key: []byte("asset:" + assetID), Value: body}).FirstErr()
}
