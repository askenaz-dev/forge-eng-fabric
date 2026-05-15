package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Active-surface validation. The how_to block describes how a human or client
// uses the asset (install per client, usage snippets per language, env reqs).
// The active_surface block describes where it is reachable at runtime: an MCP
// or A2A gateway endpoint, or an artifact pointer for skills.
//
// Schema is enforced in Go rather than via a JSON Schema engine to keep the
// dependency surface small; the rules below are the source of truth and mirror
// the matching contract in contracts/openapi/registry.yaml. Errors are
// fielded so callers can return a structured payload listing the missing or
// invalid sub-fields.

// validateHowTo returns nil if the how_to block satisfies the registry
// schema. The block must contain a non-empty install map keyed by client
// name with non-empty string commands, a usage map keyed by language with
// non-empty snippets, and an env array of required environment-variable
// names. Extra fields are permitted to allow future evolution without
// forcing a registry release.
func validateHowTo(block map[string]any) error {
	if len(block) == 0 {
		return errors.New("how_to: block is empty")
	}

	rawInstall, ok := block["install"]
	if !ok {
		return errors.New("how_to.install: required")
	}
	install, ok := rawInstall.(map[string]any)
	if !ok || len(install) == 0 {
		return errors.New("how_to.install: must be a non-empty object keyed by client")
	}
	for client, cmd := range install {
		if strings.TrimSpace(client) == "" {
			return errors.New("how_to.install: client key cannot be empty")
		}
		cmdStr, ok := cmd.(string)
		if !ok || strings.TrimSpace(cmdStr) == "" {
			return fmt.Errorf("how_to.install.%s: must be a non-empty string", client)
		}
	}

	rawUsage, ok := block["usage"]
	if !ok {
		return errors.New("how_to.usage: required")
	}
	usage, ok := rawUsage.(map[string]any)
	if !ok || len(usage) == 0 {
		return errors.New("how_to.usage: must be a non-empty object keyed by language")
	}
	for lang, snippet := range usage {
		if strings.TrimSpace(lang) == "" {
			return errors.New("how_to.usage: language key cannot be empty")
		}
		snippetStr, ok := snippet.(string)
		if !ok || strings.TrimSpace(snippetStr) == "" {
			return fmt.Errorf("how_to.usage.%s: must be a non-empty string", lang)
		}
	}

	if rawEnv, ok := block["env"]; ok {
		env, ok := rawEnv.([]any)
		if !ok {
			return errors.New("how_to.env: must be an array of strings")
		}
		for i, item := range env {
			str, ok := item.(string)
			if !ok || strings.TrimSpace(str) == "" {
				return fmt.Errorf("how_to.env[%d]: must be a non-empty string", i)
			}
		}
	}

	return nil
}

// validActiveSurfaceFamilies enumerates the three asset families that have a
// runtime invocation surface. The OPA bundles, the gateway routers and the
// Portal "How-to" tab all key off these values; keep them in lock-step with
// the corresponding enum in contracts/openapi/registry.yaml.
var validActiveSurfaceFamilies = map[string]struct{}{
	"mcp":   {},
	"a2a":   {},
	"skill": {},
}

// validateActiveSurface returns nil if the active_surface block satisfies the
// registry schema. For mcp/a2a, an endpoint URL is required. For skill, an
// artifact pointer URI + digest + signature_id are required, since skills
// are bytes that travel through pkg/artifact-store-adapter and we want the
// supply-chain pointer captured on the asset row.
func validateActiveSurface(block map[string]any) error {
	if len(block) == 0 {
		return errors.New("active_surface: block is empty")
	}

	familyRaw, ok := block["family"]
	if !ok {
		return errors.New("active_surface.family: required")
	}
	family, ok := familyRaw.(string)
	if !ok || family == "" {
		return errors.New("active_surface.family: must be a non-empty string")
	}
	if _, ok := validActiveSurfaceFamilies[family]; !ok {
		return fmt.Errorf("active_surface.family: must be one of mcp, a2a, skill (got %q)", family)
	}

	switch family {
	case "mcp", "a2a":
		endpoint, ok := block["endpoint"].(string)
		if !ok || strings.TrimSpace(endpoint) == "" {
			return fmt.Errorf("active_surface.endpoint: required for family=%s", family)
		}
		// Accept relative paths (e.g. /v1/gw/mcp/{asset_id}) and absolute URLs.
		if strings.HasPrefix(endpoint, "/") {
			return nil
		}
		if _, err := url.Parse(endpoint); err != nil {
			return fmt.Errorf("active_surface.endpoint: invalid URL: %w", err)
		}
		return nil
	case "skill":
		ptr, ok := block["artifact_pointer"].(string)
		if !ok || strings.TrimSpace(ptr) == "" {
			return errors.New("active_surface.artifact_pointer: required for family=skill")
		}
		// Pointer must include an adapter scheme (e.g. nexus://, artifactory://).
		if !strings.Contains(ptr, "://") {
			return errors.New("active_surface.artifact_pointer: must include an adapter scheme (e.g. nexus://, artifactory://)")
		}
		digest, ok := block["digest"].(string)
		if !ok || strings.TrimSpace(digest) == "" {
			return errors.New("active_surface.digest: required for family=skill")
		}
		if !strings.HasPrefix(digest, "sha256:") {
			return errors.New("active_surface.digest: must be sha256:<hex>")
		}
		sig, ok := block["signature_id"].(string)
		if !ok || strings.TrimSpace(sig) == "" {
			return errors.New("active_surface.signature_id: required for family=skill")
		}
		return nil
	}
	return nil // unreachable
}

// activeSurfaceFamilyForAssetType returns the active-surface family expected
// for a given asset type. Other types (workflow, prompt_template, etc.) have
// no runtime gateway surface today and are exempt from the precondition.
func activeSurfaceFamilyForAssetType(assetType string) (string, bool) {
	switch assetType {
	case "mcp":
		return "mcp", true
	case "agent":
		return "a2a", true
	case "skill":
		return "skill", true
	default:
		return "", false
	}
}

// validateAssetActiveSurface checks how_to + active_surface for an asset of
// the given type. It returns a structured error code suitable for the
// `code` field of a 4xx response so the Portal can surface a typed message.
//
// Returned error codes match the spec scenarios in ai-asset-registry and
// mcp-and-skills under active-registry-gateways:
//   - missing_how_to             — how_to_json is null or empty
//   - invalid_how_to             — how_to is present but fails schema
//   - missing_active_surface     — active_surface_json is null or empty
//   - invalid_active_surface     — active_surface is present but fails schema
//   - active_surface_type_mismatch — family does not match the asset's type
type validationFault struct {
	Code    string
	Message string
}

func (v *validationFault) Error() string { return v.Code + ": " + v.Message }

func validateAssetActiveSurface(assetType string, howTo, activeSurface map[string]any) *validationFault {
	if howTo == nil {
		return &validationFault{Code: "missing_how_to", Message: "how_to block is required for approved lifecycle"}
	}
	if err := validateHowTo(howTo); err != nil {
		return &validationFault{Code: "invalid_how_to", Message: err.Error()}
	}
	if activeSurface == nil {
		return &validationFault{Code: "missing_active_surface", Message: "active_surface block is required for approved lifecycle"}
	}
	if err := validateActiveSurface(activeSurface); err != nil {
		return &validationFault{Code: "invalid_active_surface", Message: err.Error()}
	}
	if expected, ok := activeSurfaceFamilyForAssetType(assetType); ok {
		family, _ := activeSurface["family"].(string)
		if family != expected {
			return &validationFault{
				Code:    "active_surface_type_mismatch",
				Message: fmt.Sprintf("asset type=%s requires active_surface.family=%s (got %s)", assetType, expected, family),
			}
		}
	}
	return nil
}

// statusForValidationFault maps validation fault codes to HTTP status codes.
// The spec calls for 409 on missing required blocks (the row exists but
// cannot be promoted yet) and 400 on invalid blocks (caller sent bad data).
func statusForValidationFault(code string) int {
	switch code {
	case "missing_how_to", "missing_active_surface", "drift_detected":
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}

// promotionPrerequisites is the subset of an asset row that every
// promotion-time gate (eval, how-to/active-surface, external-drift) reads.
// Loading it once per request keeps the SELECT footprint stable.
type promotionPrerequisites struct {
	LifecycleState string
	Type           string
	HowTo          map[string]any
	ActiveSurface  map[string]any
	Provenance     string
}

// loadPromotionPrerequisites pulls the fields needed to gate a promotion
// from the asset row. Returns pgx.ErrNoRows when the (id, version) pair
// does not exist.
func (s *server) loadPromotionPrerequisites(ctx context.Context, id, version string) (promotionPrerequisites, error) {
	return loadPromotionPrerequisitesFromPool(ctx, s.pool, id, version)
}

func loadPromotionPrerequisitesFromPool(ctx context.Context, pool *pgxpool.Pool, id, version string) (promotionPrerequisites, error) {
	var pre promotionPrerequisites
	var howTo, activeSurface []byte
	err := pool.QueryRow(ctx,
		`SELECT lifecycle_state, type, how_to_json, active_surface_json, COALESCE(external_provenance,'internal')
		 FROM asset WHERE id=$1 AND version=$2`,
		id, version,
	).Scan(&pre.LifecycleState, &pre.Type, &howTo, &activeSurface, &pre.Provenance)
	if errors.Is(err, pgx.ErrNoRows) {
		return promotionPrerequisites{}, err
	}
	if err != nil {
		return promotionPrerequisites{}, err
	}
	if len(howTo) > 0 {
		if jerr := json.Unmarshal(howTo, &pre.HowTo); jerr != nil {
			return promotionPrerequisites{}, fmt.Errorf("decode how_to_json: %w", jerr)
		}
	}
	if len(activeSurface) > 0 {
		if jerr := json.Unmarshal(activeSurface, &pre.ActiveSurface); jerr != nil {
			return promotionPrerequisites{}, fmt.Errorf("decode active_surface_json: %w", jerr)
		}
	}
	return pre, nil
}
