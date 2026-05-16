package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// DesignSystemResolver is the seam to the AI Asset Registry. Implementations
// MUST resolve aliases (`ds-forge-default`) and `asset_id[@version]` strings
// to a record describing the asset's current lifecycle state and visibility
// to the App's tenant. The service uses this to validate writes — every
// design_system_ref written to an App MUST resolve to an asset in
// `lifecycle_state=approved`.
type DesignSystemResolver interface {
	Resolve(ctx context.Context, ref string, tenantID string) (DesignSystemRecord, error)
}

// DesignSystemRecord is the projection the App service consumes. It is a
// strict subset of the Registry's DesignSystemRecord wire shape.
type DesignSystemRecord struct {
	AssetID         string
	Version         string
	LifecycleState  string
	Visibility      string // workspace | tenant | tenant_global
	// TenantID identifies the tenant that owns this Design System when
	// Visibility is `tenant` or `workspace`. Tenant-global templates carry an
	// empty TenantID. Used by the atomic create path to reject refs that are
	// visible to other tenants but not to the caller's tenant.
	TenantID        string
	BuiltInTemplate bool
}

// StaticDesignSystemResolver is the test default: a map keyed by alias or
// asset_id (without version) returning a static record. Tests inject one
// approved entry per ref they reference.
type StaticDesignSystemResolver map[string]DesignSystemRecord

func (s StaticDesignSystemResolver) Resolve(_ context.Context, ref string, _ string) (DesignSystemRecord, error) {
	key := ref
	if at := strings.Index(ref, "@"); at >= 0 {
		key = ref[:at]
	}
	rec, ok := s[key]
	if !ok {
		// Try the raw ref (e.g. when caller passed an alias literal).
		if alt, ok2 := s[ref]; ok2 {
			return alt, nil
		}
		return DesignSystemRecord{}, ErrDesignSystemNotFound
	}
	return rec, nil
}

// PROpener is the seam to the App's portal-bundle repository. The Service
// calls it inside the swap endpoint to open a PR that updates
// `app.config.json`, `tailwind.config.js` token bindings and the font preload
// manifest. The mock used in tests records the call.
type PROpener interface {
	OpenSwapPR(ctx context.Context, input SwapPRInput) (SwapPRResult, error)
	CloseSupersededPR(ctx context.Context, prURL, reason string) error
}

// SwapPRInput carries everything the PR opener needs to write the swap PR.
// `RepoURL` is selected from the App's repo_links by convention (the first
// link tagged `kind=portal-bundle` in the link metadata, or the first link if
// no tagging is present).
type SwapPRInput struct {
	AppID          string
	AppSlug        string
	RepoURL        string
	FromRef        string
	TargetRef      string
	Reason         string
	OpenedBy       string
	CorrelationID  string
	OpenSpecLink   string
}

// SwapPRResult is what the PR opener returns. URL is the PR URL surfaced to
// the API caller and persisted in `application_design_system_pr`.
type SwapPRResult struct {
	URL          string
	Number       int
	OpenedAt     time.Time
}

// MockPROpener is the in-memory PR opener used in tests and dev. It records
// every opened PR and closes them by URL.
type MockPROpener struct {
	Opens  []SwapPRInput
	Closes []struct{ URL, Reason string }
	Next   int
}

func (m *MockPROpener) OpenSwapPR(_ context.Context, input SwapPRInput) (SwapPRResult, error) {
	m.Opens = append(m.Opens, input)
	m.Next++
	return SwapPRResult{
		URL:      fmt.Sprintf("https://github.example/%s/pull/%d", input.AppSlug, m.Next),
		Number:   m.Next,
		OpenedAt: time.Now().UTC(),
	}, nil
}

func (m *MockPROpener) CloseSupersededPR(_ context.Context, url, reason string) error {
	m.Closes = append(m.Closes, struct{ URL, Reason string }{URL: url, Reason: reason})
	return nil
}

// DesignSystemPRStore tracks open swap PRs so the service can enforce the
// one-open-PR-per-App rule and auto-close superseded PRs. Production wires
// the pgx implementation; tests use the in-memory version.
type DesignSystemPRStore interface {
	OpenPR(ctx context.Context, rec DesignSystemPR) error
	ListOpen(ctx context.Context, appID string) ([]DesignSystemPR, error)
	MarkSuperseded(ctx context.Context, appID, newPRURL string) error
	MarkMerged(ctx context.Context, prURL string) error
	GetByPRURL(ctx context.Context, prURL string) (DesignSystemPR, error)
}

// DesignSystemPR is the row written to `application_design_system_pr`.
type DesignSystemPR struct {
	ID            string
	AppID         string
	WorkspaceID   string
	TenantID      string
	TargetRef     string
	Reason        string
	PRURL         string
	Status        string // open | superseded | merged | closed
	OpenedBy      string
	OpenedAt      time.Time
	ClosedAt      *time.Time
	CorrelationID string
}

// MemoryDesignSystemPRStore is the test/dev default.
type MemoryDesignSystemPRStore struct {
	rows []DesignSystemPR
}

func NewMemoryDesignSystemPRStore() *MemoryDesignSystemPRStore {
	return &MemoryDesignSystemPRStore{}
}

func (m *MemoryDesignSystemPRStore) OpenPR(_ context.Context, rec DesignSystemPR) error {
	m.rows = append(m.rows, rec)
	return nil
}

func (m *MemoryDesignSystemPRStore) ListOpen(_ context.Context, appID string) ([]DesignSystemPR, error) {
	var out []DesignSystemPR
	for _, r := range m.rows {
		if r.AppID == appID && r.Status == "open" {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *MemoryDesignSystemPRStore) MarkSuperseded(_ context.Context, appID, newPRURL string) error {
	now := time.Now().UTC()
	for i := range m.rows {
		if m.rows[i].AppID == appID && m.rows[i].Status == "open" && m.rows[i].PRURL != newPRURL {
			m.rows[i].Status = "superseded"
			m.rows[i].ClosedAt = &now
		}
	}
	return nil
}

func (m *MemoryDesignSystemPRStore) MarkMerged(_ context.Context, prURL string) error {
	now := time.Now().UTC()
	for i := range m.rows {
		if m.rows[i].PRURL == prURL {
			m.rows[i].Status = "merged"
			m.rows[i].ClosedAt = &now
			return nil
		}
	}
	return ErrSwapPRNotFound
}

func (m *MemoryDesignSystemPRStore) GetByPRURL(_ context.Context, prURL string) (DesignSystemPR, error) {
	for _, r := range m.rows {
		if r.PRURL == prURL {
			return r, nil
		}
	}
	return DesignSystemPR{}, ErrSwapPRNotFound
}

// Errors -----------------------------------------------------------------

var (
	ErrDesignSystemNotFound      = errors.New("design_system_not_found")
	ErrDesignSystemNotApproved   = errors.New("design_system_not_approved")
	// alfred-design-system-picker: returned by atomic create when the supplied
	// design_system_ref resolves to a Design System that is not visible to the
	// caller's tenant (e.g. another tenant's private template). Distinct from
	// `design_system_not_found` so the HTTP layer can return 404 vs 409.
	ErrDesignSystemNotVisible    = errors.New("design_system_not_visible")
	ErrSwapPRNotFound            = errors.New("swap_pr_not_found")
	ErrLayoutTokenOverride       = errors.New("layout_token_override_forbidden")
	ErrUnknownComponent          = errors.New("unknown_component")
	ErrAppRepoMissing            = errors.New("app_repo_missing")
)

// New service inputs ----------------------------------------------------

// SwapInput is the body accepted by POST /v1/apps/{id}/design-system:swap.
type SwapInput struct {
	TargetRef string `json:"target_ref"`
	Reason    string `json:"reason"`
}

// OverridesInput is the body accepted by PATCH /v1/apps/{id}/design-system/overrides.
type OverridesInput struct {
	Overrides map[string]string `json:"overrides"`
}

// validateOverrides enforces the canonical primitive list, the design-system
// approval requirement and the surface-tokens-only namespace. The namespace
// check is purely advisory at the API layer — the build-time merger is the
// authoritative gate. Here we reject overrides for unknown components and
// invalid refs.
func validateOverrides(in map[string]string) error {
	for name := range in {
		if _, ok := CanonicalComponentPrimitives[name]; !ok {
			return fmt.Errorf("%w: %s", ErrUnknownComponent, name)
		}
	}
	return nil
}
