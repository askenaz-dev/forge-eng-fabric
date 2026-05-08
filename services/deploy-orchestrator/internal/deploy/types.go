// Package deploy implements the deploy-orchestrator service per the
// `deploy-orchestrator`, `deployment-policies`, `image-verification-at-deploy`,
// and `deployment-history` specs.
package deploy

import (
	"sort"
	"sync"
	"time"

	"github.com/forge-eng-fabric/pkg/deployers"
	"github.com/google/uuid"
)

type Status string

const (
	StatusPending          Status = "pending"
	StatusRunning          Status = "running"
	StatusCompleted        Status = "completed"
	StatusFailed           Status = "failed"
	StatusRolledBack       Status = "rolled_back"
	StatusPendingApproval  Status = "pending_approval"
	StatusBlocked          Status = "blocked"
)

type Stage string

const (
	StageRequested    Stage = "requested"
	StagePreflight    Stage = "preflight"
	StagePolicy       Stage = "policy"
	StageImageVerify  Stage = "image_verify"
	StageRender       Stage = "render"
	StageApply        Stage = "apply"
	StageVerify       Stage = "verify"
	StageNotify       Stage = "notify"
	StageRollback     Stage = "rollback"
)

type StageOutcome string

const (
	OutcomeStarted   StageOutcome = "started"
	OutcomeCompleted StageOutcome = "completed"
	OutcomeFailed    StageOutcome = "failed"
	OutcomeDenied    StageOutcome = "denied"
	OutcomeSkipped   StageOutcome = "skipped"
)

type Deployment struct {
	ID                  string             `json:"id"`
	RequestID           string             `json:"request_id"`
	WorkspaceID         string             `json:"workspace_id"`
	TenantID            string             `json:"tenant_id"`
	AssetID             string             `json:"asset_id"`
	Env                 string             `json:"env"`
	Criticality         string             `json:"criticality"`
	DataClassification  string             `json:"data_classification,omitempty"`
	RuntimeID           string             `json:"runtime_id"`
	Image               string             `json:"image"`
	ImageDigest         string             `json:"image_digest"`
	ManifestSHA         string             `json:"manifest_sha,omitempty"`
	RevisionID          string             `json:"revision_id"`
	SourceRevisionID    string             `json:"source_revision_id,omitempty"`
	RollbackOf          string             `json:"rollback_of,omitempty"`
	OpenSpecIDs         []string           `json:"openspec_ids"`
	PRSHA               string             `json:"pr_sha,omitempty"`
	Strategy            deployers.Strategy `json:"strategy"`
	CanaryPercent       int                `json:"canary_percent,omitempty"`
	RollbackPlan        string             `json:"rollback_plan,omitempty"`
	AutoRollback        bool               `json:"auto_rollback"`
	Status              Status             `json:"status"`
	StatusReason        string             `json:"status_reason,omitempty"`
	VerifiedSignature   bool               `json:"verified_signature"`
	VerifiedAttestation bool               `json:"verified_attestation"`
	CorrelationID       string             `json:"correlation_id"`
	Actor               string             `json:"actor"`
	CreatedAt           time.Time          `json:"created_at"`
	CompletedAt         *time.Time         `json:"completed_at,omitempty"`
}

type DeploymentEvent struct {
	ID         string            `json:"id"`
	DeploymentID string          `json:"deployment_id"`
	Stage      Stage             `json:"stage"`
	Outcome    StageOutcome      `json:"outcome"`
	Reason     string            `json:"reason,omitempty"`
	Detail     map[string]any    `json:"detail,omitempty"`
	StartedAt  time.Time         `json:"started_at"`
	EndedAt    time.Time         `json:"ended_at"`
	DurationMS int64             `json:"duration_ms"`
}

type PolicyEvalResult struct {
	ID            string         `json:"id"`
	DeploymentID  string         `json:"deployment_id"`
	PolicyID      string         `json:"policy_id"`
	Outcome       string         `json:"outcome"` // allow / deny / requires_approval
	Reason        string         `json:"reason,omitempty"`
	Detail        map[string]any `json:"detail,omitempty"`
	EvaluatedAt   time.Time      `json:"evaluated_at"`
}

type ImageVerificationResult struct {
	ID                  string    `json:"id"`
	DeploymentID        string    `json:"deployment_id"`
	Outcome             string    `json:"outcome"`
	Reason              string    `json:"reason,omitempty"`
	Identity            string    `json:"identity,omitempty"`
	Digest              string    `json:"digest,omitempty"`
	SignatureVerified   bool      `json:"signature_verified"`
	AttestationVerified bool      `json:"attestation_verified"`
	OverrideID          string    `json:"override_id,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type RollbackRecord struct {
	ID            string     `json:"id"`
	DeploymentID  string     `json:"deployment_id"`
	SourceRevID   string     `json:"source_revision_id"`
	RestoredRevID string     `json:"restored_revision_id"`
	Reason        string     `json:"reason"`
	Trigger       string     `json:"trigger"` // manual | auto
	Approved      bool       `json:"approved"`
	CreatedAt     time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type DeployRequest struct {
	RequestID          string             `json:"request_id"`
	WorkspaceID        string             `json:"workspace_id"`
	TenantID           string             `json:"tenant_id"`
	AssetID            string             `json:"asset_id"`
	RuntimeID          string             `json:"runtime_id"`
	Env                string             `json:"env"`
	Criticality        string             `json:"criticality"`
	DataClassification string             `json:"data_classification,omitempty"`
	Image              string             `json:"image"`
	ImageDigest        string             `json:"image_digest"`
	OpenSpecIDs        []string           `json:"openspec_ids,omitempty"`
	PRSHA              string             `json:"pr_sha,omitempty"`
	Strategy           deployers.Strategy `json:"strategy,omitempty"`
	CanaryPercent      int                `json:"canary_percent,omitempty"`
	RollbackPlan       string             `json:"rollback_plan,omitempty"`
	AutoRollback       bool               `json:"auto_rollback"`
	Actor              string             `json:"actor"`
	Manifest           deployers.Manifest `json:"manifest"`
}

type DeployResponse struct {
	Deployment *Deployment `json:"deployment"`
	Status     string      `json:"status"`
	Reason     string      `json:"reason,omitempty"`
	Created    bool        `json:"created"`
}

type RollbackRequest struct {
	Reason   string `json:"reason"`
	Approved bool   `json:"approved"`
	Actor    string `json:"actor"`
	Manual   bool   `json:"manual"`
}

func newID() string { return uuid.NewString() }

// Store is an in-memory persistence backend mirroring
// `db/migrations/deploy-orchestrator/0001_init.sql`. It supports the
// "immutable revision history" requirement: once a Deployment is stored its
// fields are not mutated except for terminal status transitions.
type Store struct {
	mu               sync.RWMutex
	deployments      map[string]*Deployment      // by deployment id
	byRequest        map[string]string           // request_id → deployment_id (idempotency)
	events           map[string][]DeploymentEvent
	policyEvals      map[string][]PolicyEvalResult
	imageResults     map[string][]ImageVerificationResult
	rollbacks        map[string][]RollbackRecord
	revisions        map[string][]*Deployment // (asset_id|env) → ordered deployments
	listeners        map[string][]chan DeploymentEvent
}

func NewStore() *Store {
	return &Store{
		deployments:  map[string]*Deployment{},
		byRequest:    map[string]string{},
		events:       map[string][]DeploymentEvent{},
		policyEvals:  map[string][]PolicyEvalResult{},
		imageResults: map[string][]ImageVerificationResult{},
		rollbacks:    map[string][]RollbackRecord{},
		revisions:    map[string][]*Deployment{},
		listeners:    map[string][]chan DeploymentEvent{},
	}
}

func (s *Store) Insert(d *Deployment) (*Deployment, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.byRequest[d.RequestID]; ok {
		return s.deployments[existing], false
	}
	if d.ID == "" {
		d.ID = newID()
	}
	if d.CorrelationID == "" {
		d.CorrelationID = newID()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	s.deployments[d.ID] = d
	s.byRequest[d.RequestID] = d.ID
	key := d.AssetID + "|" + d.Env
	s.revisions[key] = append(s.revisions[key], d)
	return d, true
}

func (s *Store) Get(id string) (*Deployment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.deployments[id]
	if !ok {
		return nil, false
	}
	copy := *d
	return &copy, true
}

func (s *Store) List(workspaceID, env string) []*Deployment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Deployment, 0, len(s.deployments))
	for _, d := range s.deployments {
		if workspaceID != "" && d.WorkspaceID != workspaceID {
			continue
		}
		if env != "" && d.Env != env {
			continue
		}
		copy := *d
		out = append(out, &copy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// SetStatus is the only mutation allowed on Deployment after Insert. Using
// a narrow setter keeps the immutability invariant for revision history.
func (s *Store) SetStatus(id string, status Status, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.deployments[id]
	if !ok {
		return
	}
	d.Status = status
	d.StatusReason = reason
	if status == StatusCompleted || status == StatusFailed || status == StatusRolledBack {
		now := time.Now().UTC()
		d.CompletedAt = &now
	}
}

func (s *Store) SetVerified(id string, signature, attestation bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.deployments[id]
	if !ok {
		return
	}
	d.VerifiedSignature = signature
	d.VerifiedAttestation = attestation
}

func (s *Store) SetManifestSHA(id, sha string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.deployments[id]; ok {
		d.ManifestSHA = sha
	}
}

func (s *Store) AppendEvent(ev DeploymentEvent) DeploymentEvent {
	if ev.ID == "" {
		ev.ID = newID()
	}
	s.mu.Lock()
	s.events[ev.DeploymentID] = append(s.events[ev.DeploymentID], ev)
	listeners := append([]chan DeploymentEvent(nil), s.listeners[ev.DeploymentID]...)
	s.mu.Unlock()
	for _, ch := range listeners {
		select {
		case ch <- ev:
		default:
		}
	}
	return ev
}

func (s *Store) Events(deploymentID string) []DeploymentEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]DeploymentEvent(nil), s.events[deploymentID]...)
	return out
}

func (s *Store) AppendPolicyEval(r PolicyEvalResult) {
	if r.ID == "" {
		r.ID = newID()
	}
	if r.EvaluatedAt.IsZero() {
		r.EvaluatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policyEvals[r.DeploymentID] = append(s.policyEvals[r.DeploymentID], r)
}

func (s *Store) PolicyEvals(deploymentID string) []PolicyEvalResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]PolicyEvalResult(nil), s.policyEvals[deploymentID]...)
}

func (s *Store) AppendImageVerification(r ImageVerificationResult) {
	if r.ID == "" {
		r.ID = newID()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.imageResults[r.DeploymentID] = append(s.imageResults[r.DeploymentID], r)
}

func (s *Store) ImageVerifications(deploymentID string) []ImageVerificationResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ImageVerificationResult(nil), s.imageResults[deploymentID]...)
}

func (s *Store) AppendRollback(r RollbackRecord) {
	if r.ID == "" {
		r.ID = newID()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rollbacks[r.DeploymentID] = append(s.rollbacks[r.DeploymentID], r)
}

func (s *Store) Rollbacks(deploymentID string) []RollbackRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]RollbackRecord(nil), s.rollbacks[deploymentID]...)
}

// PreviousRevision returns the deployment immediately before the given one
// for the same asset+env, used by rollback flows.
func (s *Store) PreviousRevision(assetID, env, currentDepID string) *Deployment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := assetID + "|" + env
	revs := s.revisions[key]
	for i := len(revs) - 1; i >= 0; i-- {
		if revs[i].ID != currentDepID && revs[i].Status == StatusCompleted {
			copy := *revs[i]
			return &copy
		}
	}
	return nil
}

// AssetDeployments returns paginated deployments for an asset+env.
func (s *Store) AssetDeployments(assetID, env string, limit int, cursor string) ([]*Deployment, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := append([]*Deployment(nil), s.revisions[assetID+"|"+env]...)
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	start := 0
	if cursor != "" {
		for i, d := range all {
			if d.ID == cursor {
				start = i + 1
				break
			}
		}
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	end := start + limit
	if end > len(all) {
		end = len(all)
	}
	page := all[start:end]
	next := ""
	if end < len(all) {
		next = all[end-1].ID
	}
	out := make([]*Deployment, len(page))
	for i, d := range page {
		copy := *d
		out[i] = &copy
	}
	return out, next
}

func (s *Store) Subscribe(deploymentID string) (<-chan DeploymentEvent, func()) {
	ch := make(chan DeploymentEvent, 64)
	s.mu.Lock()
	s.listeners[deploymentID] = append(s.listeners[deploymentID], ch)
	s.mu.Unlock()
	cleanup := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		ls := s.listeners[deploymentID]
		for i, c := range ls {
			if c == ch {
				s.listeners[deploymentID] = append(ls[:i], ls[i+1:]...)
				break
			}
		}
		close(ch)
	}
	return ch, cleanup
}
