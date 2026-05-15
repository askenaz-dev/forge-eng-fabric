// Package application implements the App ("Application") aggregate per the
// `application-entity` spec capability (openspec/specs/application-entity).
// Apps sit between Workspace and OpenSpec in the hierarchy
// Tenant -> Workspace -> App -> Specs[].
package application

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// UnassignedSlug is the reserved slug for the system-managed App that holds
// specs that have not yet been mapped to a real App. Exactly one
// `_unassigned` App exists per workspace; the workspace-bootstrap pipeline is
// the only legitimate creator and the migration job is the only writer that
// can target it as `app_id`. See Decision 5 of the design.
const UnassignedSlug = "_unassigned"

// SystemActor is the principal name used by the platform when it creates
// system-managed resources (the `_unassigned` App, migration backfill rows).
// Real callers MUST never present this principal — handlers reject it.
const SystemActor = "system:forge-platform"

// Lifecycle states an App can be in. `deleted` is terminal and only used for
// audit trail bookkeeping; the row is hard-deleted from the live `application`
// table on successful DELETE, but the audit record carries the prior body.
type Lifecycle string

const (
	LifecycleActive   Lifecycle = "active"
	LifecycleArchived Lifecycle = "archived"
	LifecycleDeleted  Lifecycle = "deleted"
)

// TargetValue is one of the four allowed per-phase policy values.
type TargetValue string

const (
	TargetRequired TargetValue = "required"
	TargetOptional TargetValue = "optional"
	TargetOptIn    TargetValue = "opt-in"
	TargetSkipped  TargetValue = "skipped"
)

// AllTargetValues is the exhaustive set of allowed values for any phase key.
var AllTargetValues = map[TargetValue]struct{}{
	TargetRequired: {},
	TargetOptional: {},
	TargetOptIn:    {},
	TargetSkipped:  {},
}

// AllTargetPhases is the exhaustive set of phase keys the platform recognises.
var AllTargetPhases = map[string]struct{}{
	"architect":    {},
	"design":       {},
	"development":  {},
	"qa":           {},
	"security":     {},
	"devops":       {},
	"iac":          {},
	"sre":          {},
	"finops":       {},
	"observability": {},
}

// DefaultTargets returns the platform-default targets map applied to every
// new App per the sdlc-end-to-end spec.
func DefaultTargets() map[string]TargetValue {
	return map[string]TargetValue{
		"architect":    TargetRequired,
		"design":       TargetOptional,
		"development":  TargetRequired,
		"qa":           TargetRequired,
		"security":     TargetRequired,
		"devops":       TargetRequired,
		"iac":          TargetOptIn,
		"sre":          TargetOptional,
		"finops":       TargetOptIn,
		"observability": TargetOptIn,
	}
}

// App is the canonical wire-and-store representation of an Application.
type App struct {
	ID                    string                 `json:"id"`
	Slug                  string                 `json:"slug"`
	Name                  string                 `json:"name"`
	Description           string                 `json:"description"`
	WorkspaceID           string                 `json:"workspace_id"`
	TenantID              string                 `json:"tenant_id"`
	Lifecycle             Lifecycle              `json:"lifecycle_state"`
	DesignSystemRef       string                 `json:"design_system_ref"`
	// DesignSystemOverrides maps component primitives (button, card, ...) to a
	// secondary design_system_ref. Layout-token overrides are forbidden at the
	// build-time merger; the API only validates membership in the canonical
	// primitive list at the wire boundary.
	DesignSystemOverrides map[string]string      `json:"design_system_overrides"`
	DefaultEnvironments   []string               `json:"default_environments"`
	RepoLinks             []string               `json:"repo_links"`
	RuntimeLinks          []string               `json:"runtime_links"`
	Owners                []string               `json:"owners"`
	// Targets declares the per-phase SDLC delivery policy for this App.
	// Keys are the canonical phase names; values are TargetValue constants.
	Targets               map[string]TargetValue `json:"targets"`
	SystemManaged         bool                   `json:"system_managed"`
	CreatedAt             time.Time              `json:"created_at"`
	CreatedBy             string                 `json:"created_by"`
	UpdatedAt             time.Time              `json:"updated_at"`
	UpdatedBy             string                 `json:"updated_by,omitempty"`
	ArchivedAt            *time.Time             `json:"archived_at,omitempty"`
}

// DesignSystemDefaultRef is the alias every App falls back to when no
// explicit ref is supplied at creation time or by the migration backfill.
const DesignSystemDefaultRef = "ds-forge-default"

// CanonicalComponentPrimitives is the exhaustive list of component names the
// `PATCH /v1/apps/{id}/design-system/overrides` endpoint accepts. Overrides
// for any other name are rejected with `422 unknown_component`.
var CanonicalComponentPrimitives = map[string]struct{}{
	"button":        {},
	"badge":         {},
	"card":          {},
	"kpi":           {},
	"chip":          {},
	"seg":           {},
	"sheet":         {},
	"terminal":      {},
	"code":          {},
	"run_row":       {},
	"approval_card": {},
}

// CreateInput is the payload accepted by POST /v1/workspaces/{ws}/apps.
type CreateInput struct {
	Slug                string   `json:"slug"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Owners              []string `json:"owners"`
	DesignSystemRef     string   `json:"design_system_ref,omitempty"`
	DefaultEnvironments []string `json:"default_environments,omitempty"`
	RepoLinks           []string `json:"repo_links,omitempty"`
	RuntimeLinks        []string `json:"runtime_links,omitempty"`
}

// PatchInput is the payload accepted by PATCH /v1/apps/{id}. Pointers are used
// to distinguish "not provided" from "set to empty". Slug and WorkspaceID
// cannot be patched after creation.
type PatchInput struct {
	Name                *string                `json:"name,omitempty"`
	Description         *string                `json:"description,omitempty"`
	Owners              *[]string              `json:"owners,omitempty"`
	DesignSystemRef     *string                `json:"design_system_ref,omitempty"`
	DefaultEnvironments *[]string              `json:"default_environments,omitempty"`
	RepoLinks           *[]string              `json:"repo_links,omitempty"`
	RuntimeLinks        *[]string              `json:"runtime_links,omitempty"`
	// Targets patches individual phase targets. Omitted keys are preserved;
	// unknown phase keys or disallowed values are rejected with 422.
	Targets             map[string]TargetValue `json:"targets,omitempty"`
}

// Filter is the predicate accepted by Store.List.
type Filter struct {
	WorkspaceID     string
	IncludeArchived bool
	IncludeDeleted  bool
}

// Error codes returned by the service layer. Handlers map these to HTTP
// status codes. Keep these stable: spec scenarios and Portal copy refer to
// them by name.
var (
	ErrSlugConflict          = errors.New("app_slug_conflict")
	ErrSlugInvalid           = errors.New("app_slug_invalid")
	ErrSystemManaged         = errors.New("system_managed_app")
	ErrAppNotFound           = errors.New("app_not_found")
	ErrAppArchived           = errors.New("app_archived")
	ErrAppHasLiveArtefacts   = errors.New("app_has_live_artefacts")
	ErrAppWorkspaceMismatch  = errors.New("app_workspace_mismatch")
	ErrMissingOwners         = errors.New("missing_owners")
	ErrMissingName           = errors.New("missing_name")
	ErrSlugReserved          = errors.New("app_slug_reserved")
	ErrForbidden             = errors.New("forbidden")
	ErrInvalidTargetPhase    = errors.New("invalid_target_phase")
	ErrInvalidTargetValue    = errors.New("invalid_target_value")
)

// ValidateTargets checks that all keys are known phase names and all values
// are allowed TargetValues. Returns the first offending key/value via the
// structured error type TargetValidationError.
func ValidateTargets(targets map[string]TargetValue) error {
	for phase, val := range targets {
		if _, ok := AllTargetPhases[phase]; !ok {
			return &TargetValidationError{Phase: phase, Value: string(val), Code: ErrInvalidTargetPhase}
		}
		if _, ok := AllTargetValues[val]; !ok {
			return &TargetValidationError{Phase: phase, Value: string(val), Code: ErrInvalidTargetValue}
		}
	}
	return nil
}

// TargetValidationError carries the offending phase+value pair so the HTTP
// handler can return a descriptive 422 body.
type TargetValidationError struct {
	Phase string
	Value string
	Code  error
}

func (e *TargetValidationError) Error() string { return e.Code.Error() }
func (e *TargetValidationError) Unwrap() error { return e.Code }

// slugPattern matches kebab/snake-case identifiers and forbids leading uppercase
// or dots. Starts with letter/digit, then any of [a-z0-9_-], 1-63 chars.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// IsReservedSlug reports whether `slug` is reserved for system use. Currently
// any slug starting with `_` is reserved (default policy from design Open Q).
func IsReservedSlug(slug string) bool {
	return strings.HasPrefix(slug, "_")
}

// ValidateSlug enforces the slug grammar. Reserved slugs are *not* rejected
// here because the bootstrap path needs to create `_unassigned`; callers that
// need the reserved-slug check do it separately via IsReservedSlug.
func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) {
		return ErrSlugInvalid
	}
	return nil
}

func newID() string { return uuid.NewString() }

// Store is the in-memory persistence backend. It exists so unit tests can run
// without Postgres; production wires the same interface to pgx-backed code in
// internal/store. See db/migrations/registry/0008_application_entity.sql for
// the matching schema.
type Store struct {
	mu       sync.RWMutex
	apps     map[string]*App   // id -> App
	bySlug   map[string]string // workspace_id + "/" + slug -> app id
}

func NewStore() *Store {
	return &Store{
		apps:   map[string]*App{},
		bySlug: map[string]string{},
	}
}

func slugKey(workspaceID, slug string) string { return workspaceID + "/" + slug }

// Insert persists a new App. Returns ErrSlugConflict if (workspace, slug) is
// already taken.
func (s *Store) Insert(app *App) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.bySlug[slugKey(app.WorkspaceID, app.Slug)]; exists {
		return nil, ErrSlugConflict
	}
	if app.ID == "" {
		app.ID = newID()
	}
	now := time.Now().UTC()
	if app.CreatedAt.IsZero() {
		app.CreatedAt = now
	}
	if app.UpdatedAt.IsZero() {
		app.UpdatedAt = app.CreatedAt
	}
	if app.Lifecycle == "" {
		app.Lifecycle = LifecycleActive
	}
	if app.DefaultEnvironments == nil {
		app.DefaultEnvironments = []string{}
	}
	if app.RepoLinks == nil {
		app.RepoLinks = []string{}
	}
	if app.RuntimeLinks == nil {
		app.RuntimeLinks = []string{}
	}
	if app.DesignSystemRef == "" {
		app.DesignSystemRef = DesignSystemDefaultRef
	}
	if app.DesignSystemOverrides == nil {
		app.DesignSystemOverrides = map[string]string{}
	}
	if app.Targets == nil {
		app.Targets = DefaultTargets()
	}
	clone := *app
	s.apps[app.ID] = &clone
	s.bySlug[slugKey(app.WorkspaceID, app.Slug)] = app.ID
	out := clone
	return &out, nil
}

// Get returns a copy of the App with the given id, or ErrAppNotFound.
func (s *Store) Get(id string) (*App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.apps[id]
	if !ok || app.Lifecycle == LifecycleDeleted {
		return nil, ErrAppNotFound
	}
	clone := *app
	return &clone, nil
}

// GetBySlug returns the App matching the (workspace, slug) pair or ErrAppNotFound.
func (s *Store) GetBySlug(workspaceID, slug string) (*App, error) {
	s.mu.RLock()
	id, ok := s.bySlug[slugKey(workspaceID, slug)]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrAppNotFound
	}
	return s.Get(id)
}

// List returns Apps matching the filter, sorted by created_at descending.
// `_unassigned` is included only when the filter's workspace matches.
func (s *Store) List(filter Filter) []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*App, 0, len(s.apps))
	for _, app := range s.apps {
		if filter.WorkspaceID != "" && app.WorkspaceID != filter.WorkspaceID {
			continue
		}
		if !filter.IncludeArchived && app.Lifecycle == LifecycleArchived {
			continue
		}
		if !filter.IncludeDeleted && app.Lifecycle == LifecycleDeleted {
			continue
		}
		clone := *app
		out = append(out, &clone)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// Update applies the patch to an App. Slug, WorkspaceID, TenantID and
// SystemManaged cannot be patched.
func (s *Store) Update(id string, mutate func(*App) error) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.apps[id]
	if !ok || app.Lifecycle == LifecycleDeleted {
		return nil, ErrAppNotFound
	}
	clone := *app
	if err := mutate(&clone); err != nil {
		return nil, err
	}
	clone.UpdatedAt = time.Now().UTC()
	s.apps[id] = &clone
	out := clone
	return &out, nil
}

// Delete hard-deletes the App from the live table. Callers (the service
// layer) MUST verify there are no live artefacts before invoking this.
func (s *Store) Delete(id string) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.apps[id]
	if !ok {
		return nil, ErrAppNotFound
	}
	delete(s.apps, id)
	delete(s.bySlug, slugKey(app.WorkspaceID, app.Slug))
	clone := *app
	clone.Lifecycle = LifecycleDeleted
	return &clone, nil
}
