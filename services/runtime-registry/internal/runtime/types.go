// Package runtime implements the runtime registry per the
// `runtime-connectors`, `byo-runtime-onboarding`, and
// `forge-provisioned-runtime` specs.
package runtime

import (
	"sort"
	"sync"
	"time"

	rt "github.com/forge-eng-fabric/pkg/runtimes"
	"github.com/google/uuid"
)

// Re-export shared types/constants so existing call sites keep working.
type (
	Type         = rt.Type
	Mode         = rt.Mode
	Visibility   = rt.Visibility
	GKEMode      = rt.GKEMode
	Capabilities = rt.Capabilities
	Runtime      = rt.Runtime
)

const (
	TypeGKE      = rt.TypeGKE
	TypeCloudRun = rt.TypeCloudRun
	TypeMinikube = rt.TypeMinikube

	ModeBYO         = rt.ModeBYO
	ModeProvisioned = rt.ModeProvisioned

	VisibilityWorkspace = rt.VisibilityWorkspace
	VisibilityTenant    = rt.VisibilityTenant

	GKEStandard  = rt.GKEStandard
	GKEAutopilot = rt.GKEAutopilot
)

func DefaultCapabilities(t Type) Capabilities { return rt.DefaultCapabilities(t) }

type PreflightOutcome string

const (
	PreflightSuccess PreflightOutcome = "success"
	PreflightFailed  PreflightOutcome = "failed"
)

type PreflightCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Reason  string `json:"reason,omitempty"`
	Details string `json:"details,omitempty"`
}

type PreflightResult struct {
	ID        string           `json:"id"`
	RuntimeID string           `json:"runtime_id"`
	Outcome   PreflightOutcome `json:"outcome"`
	Reason    string           `json:"reason,omitempty"`
	Checks    []PreflightCheck `json:"checks"`
	StartedAt time.Time        `json:"started_at"`
	EndedAt   time.Time        `json:"ended_at"`
	Metadata  map[string]any   `json:"metadata,omitempty"`
}

type RegisterRequest struct {
	WorkspaceID         string         `json:"workspace_id"`
	TenantID            string         `json:"tenant_id"`
	Type                Type           `json:"type"`
	Mode                Mode           `json:"mode"`
	Visibility          Visibility     `json:"visibility,omitempty"`
	Name                string         `json:"name"`
	Region              string         `json:"region,omitempty"`
	GKEMode             GKEMode        `json:"gke_mode,omitempty"`
	ProjectID           string         `json:"project_id,omitempty"`
	ClusterName         string         `json:"cluster_name,omitempty"`
	Endpoint            string         `json:"endpoint,omitempty"`
	ServiceAccountEmail string         `json:"service_account_email,omitempty"`
	Namespace           string         `json:"namespace,omitempty"`
	Kubeconfig          string         `json:"kubeconfig,omitempty"`
	SAKey               string         `json:"sa_key,omitempty"`
	Labels              map[string]any `json:"labels,omitempty"`
}

type RegisterResponse struct {
	Runtime *Runtime `json:"runtime"`
}

type ProvisionRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	TenantID    string  `json:"tenant_id"`
	Type        Type    `json:"type"`
	Name        string  `json:"name"`
	Region      string  `json:"region,omitempty"`
	GKEMode     GKEMode `json:"gke_mode,omitempty"`
	Env         string  `json:"env,omitempty"`
}

type ProvisionResponse struct {
	Runtime *Runtime         `json:"runtime"`
	Outputs map[string]any   `json:"outputs"`
	Plan    []TerraformEvent `json:"plan_events,omitempty"`
}

type TerraformEvent struct {
	Type     string         `json:"type"`
	Resource string         `json:"resource,omitempty"`
	Action   string         `json:"action,omitempty"`
	Outcome  string         `json:"outcome"`
	Outputs  map[string]any `json:"outputs,omitempty"`
}

func newID() string { return uuid.NewString() }

// Store is the in-memory persistence backend mirroring
// `db/migrations/runtime-registry/0001_init.sql`.
type Store struct {
	mu        sync.RWMutex
	runtimes  map[string]*Runtime
	preflight map[string][]*PreflightResult
}

func NewStore() *Store {
	return &Store{
		runtimes:  map[string]*Runtime{},
		preflight: map[string][]*PreflightResult{},
	}
}

func (s *Store) Insert(r *Runtime) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = newID()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	r.UpdatedAt = time.Now().UTC()
	s.runtimes[r.ID] = r
	return nil
}

func (s *Store) Get(id string) (*Runtime, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runtimes[id]
	if !ok {
		return nil, false
	}
	copy := *r
	return &copy, true
}

func (s *Store) Update(r *Runtime) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.UpdatedAt = time.Now().UTC()
	s.runtimes[r.ID] = r
}

func (s *Store) List(workspaceID string) []*Runtime {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Runtime, 0, len(s.runtimes))
	for _, r := range s.runtimes {
		if workspaceID != "" && r.WorkspaceID != workspaceID {
			continue
		}
		copy := *r
		out = append(out, &copy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (s *Store) AppendPreflight(result *PreflightResult) {
	if result.ID == "" {
		result.ID = newID()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.preflight[result.RuntimeID] = append(s.preflight[result.RuntimeID], result)
}

func (s *Store) Preflights(runtimeID string) []*PreflightResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]*PreflightResult(nil), s.preflight[runtimeID]...)
	return out
}

// Delete removes a runtime entirely (used by tests / admin). Production
// favors `Revoke` instead.
func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runtimes, id)
	delete(s.preflight, id)
}
