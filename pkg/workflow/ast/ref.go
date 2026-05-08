package ast

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// AssetRef parses references of the form
//
//	registry:<asset-type>/<id>@<version>
//
// e.g. `registry:skill/sdlc-product/refine-user-story@1.2.0`.
type AssetRef struct {
	Type    string // skill, mcp, prompt, eval_dataset, workflow
	ID      string // arbitrary path-segment id, may include namespace
	Version string // SemVer; floating tags rejected
}

var (
	ErrNotRegistryRef     = errors.New("not_a_registry_reference")
	ErrFloatingReference  = errors.New("floating_reference_not_allowed")
	ErrMalformedReference = errors.New("malformed_reference")
	semverRefPattern      = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	floatingTagWords      = map[string]struct{}{
		"latest":  {},
		"main":    {},
		"master":  {},
		"current": {},
		"stable":  {},
	}
	// mcpPermissionScopes are the permission modes that MAY appear in place
	// of a SemVer for MCP server references (e.g. registry:mcp/github@write).
	mcpPermissionScopes = map[string]struct{}{
		"read":  {},
		"write": {},
		"admin": {},
	}
)

// ParseAssetRef parses a `registry:<type>/<id>@<version>` reference.
// Floating tags such as `latest` are explicitly rejected.
//
// Special case: MCP server references may use a permission scope
// (`read`, `write`, `admin`) in place of SemVer; the actual version of the
// MCP image is determined at runtime by the runtime registry.
func ParseAssetRef(ref string) (AssetRef, error) {
	if !strings.HasPrefix(ref, "registry:") {
		return AssetRef{}, ErrNotRegistryRef
	}
	body := strings.TrimPrefix(ref, "registry:")
	at := strings.LastIndex(body, "@")
	if at <= 0 || at == len(body)-1 {
		return AssetRef{}, ErrMalformedReference
	}
	left, right := body[:at], body[at+1:]
	slash := strings.Index(left, "/")
	if slash < 0 {
		return AssetRef{}, ErrMalformedReference
	}
	if slash == 0 || slash == len(left)-1 {
		return AssetRef{}, ErrMalformedReference
	}
	out := AssetRef{
		Type:    left[:slash],
		ID:      left[slash+1:],
		Version: right,
	}
	if _, isFloat := floatingTagWords[strings.ToLower(out.Version)]; isFloat {
		return out, ErrFloatingReference
	}
	if out.Type == "mcp" {
		if _, ok := mcpPermissionScopes[strings.ToLower(out.Version)]; ok {
			return out, nil
		}
	}
	if !semverRefPattern.MatchString(out.Version) {
		return out, ErrFloatingReference
	}
	return out, nil
}

// String renders the canonical form.
func (r AssetRef) String() string {
	return fmt.Sprintf("registry:%s/%s@%s", r.Type, r.ID, r.Version)
}
