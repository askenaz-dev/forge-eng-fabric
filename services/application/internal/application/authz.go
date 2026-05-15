package application

import (
	"context"
	"sync"
)

// Permission is the relation we ask OpenFGA about. It mirrors the `app` type
// relations declared in contracts/openfga/authorization-model.json.
type Permission string

const (
	PermView     Permission = "can_view"
	PermEdit     Permission = "can_edit"
	PermAdmin    Permission = "can_admin"
	PermOwner    Permission = "owner"
	PermEditor   Permission = "editor"
	PermViewer   Permission = "viewer"
	PermWSEditor Permission = "workspace_editor" // pseudo-permission resolved on workspace, used for `POST /v1/workspaces/{ws}/apps`
)

// Authorizer is the seam to OpenFGA. The contract intentionally accepts the
// object string (e.g. `app:<id>` or `workspace:<id>`) so callers can ask for
// either App-scoped or Workspace-scoped checks without us hard-coding object
// types here.
type Authorizer interface {
	Check(ctx context.Context, user, relation, object string) (bool, error)
	WriteTuple(ctx context.Context, user, relation, object string) error
	DeleteTuple(ctx context.Context, user, relation, object string) error
}

// AllowAllAuthorizer is the dev/test default and the implementation used when
// no OpenFGA URL is configured. It logs into MemoryAuthorizer.tuples for
// assertion purposes.
type AllowAllAuthorizer struct {
	mu     sync.Mutex
	tuples []Tuple
}

// Tuple captures a write to OpenFGA. Tests assert against this to verify that
// the service wires `app#owner` tuples on creation, etc.
type Tuple struct {
	User     string
	Relation string
	Object   string
}

func NewAllowAllAuthorizer() *AllowAllAuthorizer { return &AllowAllAuthorizer{} }

func (a *AllowAllAuthorizer) Check(_ context.Context, _, _, _ string) (bool, error) {
	return true, nil
}

func (a *AllowAllAuthorizer) WriteTuple(_ context.Context, user, relation, object string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tuples = append(a.tuples, Tuple{User: user, Relation: relation, Object: object})
	return nil
}

func (a *AllowAllAuthorizer) DeleteTuple(_ context.Context, user, relation, object string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, t := range a.tuples {
		if t.User == user && t.Relation == relation && t.Object == object {
			a.tuples = append(a.tuples[:i], a.tuples[i+1:]...)
			break
		}
	}
	return nil
}

func (a *AllowAllAuthorizer) Tuples() []Tuple {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]Tuple, len(a.tuples))
	copy(out, a.tuples)
	return out
}

// LiveArtefactChecker abstracts the cross-service lookup that determines
// whether an App still has live artefacts (specs in non-terminal state,
// deployments, in-flight onboarding requests, registered runtimes). The
// service uses this to enforce the `app_has_live_artefacts` invariant on
// DELETE. Production implementations issue parallel queries against the
// openspec, app-onboarding, registry and runtime-registry services; tests
// inject a static fake.
type LiveArtefactChecker interface {
	Check(ctx context.Context, appID string) (LiveArtefactReport, error)
}

// LiveArtefactReport details the live artefacts blocking deletion. Empty
// slices mean no blockers.
type LiveArtefactReport struct {
	Specs       []string `json:"specs,omitempty"`
	Deployments []string `json:"deployments,omitempty"`
	Onboarding  []string `json:"onboarding,omitempty"`
	Runtimes    []string `json:"runtimes,omitempty"`
}

func (r LiveArtefactReport) Empty() bool {
	return len(r.Specs) == 0 && len(r.Deployments) == 0 && len(r.Onboarding) == 0 && len(r.Runtimes) == 0
}

// NoLiveArtefacts is the test/dev default — always reports no blockers.
type NoLiveArtefacts struct{}

func (NoLiveArtefacts) Check(_ context.Context, _ string) (LiveArtefactReport, error) {
	return LiveArtefactReport{}, nil
}

// StaticLiveArtefacts returns a fixed report. Useful in tests when verifying
// that DELETE refuses with `app_has_live_artefacts`.
type StaticLiveArtefacts struct{ Report LiveArtefactReport }

func (s StaticLiveArtefacts) Check(_ context.Context, _ string) (LiveArtefactReport, error) {
	return s.Report, nil
}

// WorkspaceLookup is the seam that returns the tenant_id for a workspace. The
// service needs it so it can stamp the right tenant on new Apps without
// requiring the caller to provide it.
type WorkspaceLookup interface {
	TenantID(ctx context.Context, workspaceID string) (string, error)
}

// StaticWorkspaceLookup is a map-based implementation for dev and tests.
type StaticWorkspaceLookup map[string]string

func (s StaticWorkspaceLookup) TenantID(_ context.Context, workspaceID string) (string, error) {
	tenantID, ok := s[workspaceID]
	if !ok {
		return "", ErrAppWorkspaceMismatch
	}
	return tenantID, nil
}
