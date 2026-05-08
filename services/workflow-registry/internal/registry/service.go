package registry

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
	"github.com/forge-eng-fabric/pkg/workflow/dsl"
	"github.com/forge-eng-fabric/pkg/workflow/lint"
	"github.com/forge-eng-fabric/pkg/workflow/schema"
)

var (
	ErrWorkflowNotFound       = errors.New("workflow_not_found")
	ErrVersionAlreadyExists   = errors.New("version_already_exists")
	ErrLintFailed             = errors.New("lint_failed")
	ErrSchemaValidationFailed = errors.New("schema_validation_failed")
	ErrInvalidVersion         = errors.New("invalid_version")
	ErrBreakingChange         = errors.New("breaking_change_requires_major_bump")
)

// LifecycleState mirrors the asset registry conventions.
type LifecycleState string

const (
	LifecycleDraft     LifecycleState = "draft"
	LifecycleInReview  LifecycleState = "in_review"
	LifecycleApproved  LifecycleState = "approved"
	LifecyclePublished LifecycleState = "published"
	LifecycleDeprecated LifecycleState = "deprecated"
)

// Workflow is the parent record (id-level metadata). Its versions are stored
// separately and are immutable.
type Workflow struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	WorkspaceID string         `json:"workspace_id"`
	Name        string         `json:"name"`
	Owners      []string       `json:"owners,omitempty"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Visibility  ast.Visibility `json:"visibility"`
	Latest      string         `json:"latest_version,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Version is an immutable, published-or-draft snapshot of a workflow.
type Version struct {
	WorkflowID  string         `json:"workflow_id"`
	Version     string         `json:"version"`
	AST         *ast.Workflow  `json:"ast"`
	State       LifecycleState `json:"lifecycle_state"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	CreatedBy   string         `json:"created_by,omitempty"`
	DiffPrev    *DiffResult    `json:"diff_prev,omitempty"`
}

// CreateWorkflowRequest creates the parent workflow record.
type CreateWorkflowRequest struct {
	ID          string         `json:"id"`
	TenantID    string         `json:"tenant_id"`
	WorkspaceID string         `json:"workspace_id"`
	Name        string         `json:"name"`
	Owners      []string       `json:"owners,omitempty"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Visibility  ast.Visibility `json:"visibility,omitempty"`
}

// PublishVersionRequest creates a new immutable version.
type PublishVersionRequest struct {
	TenantID     string `json:"tenant_id"`
	WorkflowID   string `json:"workflow_id"`
	WorkflowYAML string `json:"workflow_yaml"`
	Actor        string `json:"actor,omitempty"`
	// AutoBump=true: derive version from diff against latest.
	// false: use the version embedded in the YAML metadata.
	AutoBump bool `json:"auto_bump,omitempty"`
}

// Service is the registry service.
type Service struct {
	mu        sync.RWMutex
	workflows map[string]*Workflow
	versions  map[string]map[string]*Version // workflowID -> version -> Version
	now       func() time.Time
	sink      Sink
}

// NewService creates a fresh service.
func NewService(sink Sink) *Service {
	if sink == nil {
		sink = &MemorySink{}
	}
	return &Service{
		workflows: map[string]*Workflow{},
		versions:  map[string]map[string]*Version{},
		now:       func() time.Time { return time.Now().UTC() },
		sink:      sink,
	}
}

// CreateWorkflow registers a parent workflow.
func (s *Service) CreateWorkflow(_ context.Context, req CreateWorkflowRequest) (*Workflow, error) {
	if req.ID == "" {
		return nil, errors.New("id_required")
	}
	if req.Name == "" {
		return nil, errors.New("name_required")
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = ast.VisibilityPrivate
	}
	wf := &Workflow{
		ID:          req.ID,
		TenantID:    req.TenantID,
		WorkspaceID: req.WorkspaceID,
		Name:        req.Name,
		Owners:      append([]string(nil), req.Owners...),
		Description: req.Description,
		Tags:        append([]string(nil), req.Tags...),
		Visibility:  visibility,
		CreatedAt:   s.now(),
		UpdatedAt:   s.now(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.workflows[req.ID]; exists {
		return nil, fmt.Errorf("workflow_already_exists:%s", req.ID)
	}
	s.workflows[req.ID] = cloneWorkflow(wf)
	s.versions[req.ID] = map[string]*Version{}
	_ = s.sink.Emit(newEvent(wf.TenantID, wf.WorkspaceID, "workflow.created.v1", wf.ID, map[string]any{
		"workflow_id": wf.ID, "name": wf.Name, "visibility": wf.Visibility,
	}))
	return cloneWorkflow(wf), nil
}

// GetWorkflow returns a workflow by id.
func (s *Service) GetWorkflow(id string) (*Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wf, ok := s.workflows[id]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	return cloneWorkflow(wf), nil
}

// ListWorkflows returns workflows scoped to tenant/workspace.
func (s *Service) ListWorkflows(tenantID, workspaceID string) []*Workflow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []*Workflow{}
	for _, wf := range s.workflows {
		if tenantID != "" && wf.TenantID != tenantID {
			continue
		}
		if workspaceID != "" && wf.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, cloneWorkflow(wf))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// ListVersions returns sorted versions (latest first).
func (s *Service) ListVersions(workflowID string) []*Version {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions, ok := s.versions[workflowID]
	if !ok {
		return nil
	}
	out := make([]*Version, 0, len(versions))
	for _, v := range versions {
		out = append(out, cloneVersion(v))
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := ParseSemVer(out[i].Version)
		b, _ := ParseSemVer(out[j].Version)
		return a.Compare(b) > 0
	})
	return out
}

// GetVersion returns a single version.
func (s *Service) GetVersion(workflowID, version string) (*Version, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions, ok := s.versions[workflowID]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	v, ok := versions[version]
	if !ok {
		return nil, fmt.Errorf("version_not_found:%s", version)
	}
	return cloneVersion(v), nil
}

// PublishVersion validates, lints, computes the diff, enforces SemVer rules
// and stores an immutable record. Returns the created Version.
func (s *Service) PublishVersion(_ context.Context, req PublishVersionRequest) (*Version, error) {
	wfAST, err := dsl.Parse([]byte(req.WorkflowYAML))
	if err != nil {
		return nil, err
	}
	if err := schema.Validate(wfAST); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSchemaValidationFailed, err)
	}
	lintResult := lint.Lint(wfAST)
	if errs := lintResult.Errors(); len(errs) > 0 {
		return nil, fmt.Errorf("%w: %d findings", ErrLintFailed, len(errs))
	}
	requested, err := ParseSemVer(wfAST.Metadata.Version)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidVersion, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	wf, ok := s.workflows[req.WorkflowID]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	versions := s.versions[req.WorkflowID]

	// Compute diff against the latest version (if any).
	var diff *DiffResult
	var prevAST *ast.Workflow
	if wf.Latest != "" {
		prev := versions[wf.Latest]
		if prev != nil {
			prevAST = prev.AST
			d := DiffWorkflows(prevAST, wfAST)
			diff = &d
		}
	}

	// Immutability check first: a non-AutoBump request targeting a
	// published version is rejected before SemVer enforcement runs.
	if !req.AutoBump {
		if existing, exists := versions[requested.String()]; exists && existing.State == LifecyclePublished {
			return nil, fmt.Errorf("%w: %s", ErrVersionAlreadyExists, requested)
		}
	}

	// AutoBump derives the version; otherwise the requested version must
	// satisfy the diff classification.
	finalVersion := requested
	if req.AutoBump && prevAST != nil {
		prev, _ := ParseSemVer(wf.Latest)
		finalVersion = MinNextVersion(prev, diff.Bump)
		wfAST.Metadata.Version = finalVersion.String()
	} else if prevAST != nil && diff != nil {
		prev, _ := ParseSemVer(wf.Latest)
		min := MinNextVersion(prev, diff.Bump)
		if requested.Compare(min) < 0 {
			return nil, fmt.Errorf("%w: requested=%s required=%s reasons=%v",
				ErrBreakingChange, requested, min, diff.Reasons)
		}
	}

	// Recheck after possible auto-bump.
	if existing, exists := versions[finalVersion.String()]; exists && existing.State == LifecyclePublished {
		return nil, fmt.Errorf("%w: %s", ErrVersionAlreadyExists, finalVersion)
	}

	now := s.now()
	v := &Version{
		WorkflowID:  req.WorkflowID,
		Version:     finalVersion.String(),
		AST:         wfAST,
		State:       LifecyclePublished,
		PublishedAt: &now,
		CreatedAt:   now,
		CreatedBy:   req.Actor,
		DiffPrev:    diff,
	}
	versions[v.Version] = cloneVersion(v)
	s.versions[req.WorkflowID] = versions

	wf.Latest = v.Version
	wf.UpdatedAt = now
	s.workflows[req.WorkflowID] = cloneWorkflow(wf)

	_ = s.sink.Emit(newEvent(wf.TenantID, wf.WorkspaceID, "workflow.published.v1", wf.ID, map[string]any{
		"workflow_id": wf.ID,
		"version":     v.Version,
		"diff":        diff,
	}))

	return cloneVersion(v), nil
}

func cloneWorkflow(in *Workflow) *Workflow {
	if in == nil {
		return nil
	}
	out := *in
	out.Owners = append([]string(nil), in.Owners...)
	out.Tags = append([]string(nil), in.Tags...)
	return &out
}

func cloneVersion(in *Version) *Version {
	if in == nil {
		return nil
	}
	out := *in
	if in.AST != nil {
		ast := *in.AST
		out.AST = &ast
	}
	if in.DiffPrev != nil {
		d := *in.DiffPrev
		d.Reasons = append([]string(nil), in.DiffPrev.Reasons...)
		out.DiffPrev = &d
	}
	return &out
}
