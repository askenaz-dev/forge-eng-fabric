package deploy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/forge-eng-fabric/pkg/cosign"
	"github.com/forge-eng-fabric/pkg/deployers"
	rt "github.com/forge-eng-fabric/pkg/runtimes"
)

var (
	ErrDeploymentNotFound = errors.New("deployment_not_found")
	ErrRuntimeRequired    = errors.New("runtime_required")
	ErrRuntimeNotFound    = errors.New("runtime_not_found")
	ErrCrossWorkspace     = errors.New("cross_workspace_denied")
	ErrRuntimeRevoked     = errors.New("runtime_revoked")
	ErrPolicyDenied       = errors.New("policy_denied")
	ErrPendingApproval    = errors.New("pending_approval")
	ErrImageVerification  = errors.New("image_verification_failed")
	ErrDeployerMissing    = errors.New("deployer_missing")
	ErrPreviousRevision   = errors.New("no_previous_revision")
	ErrNonReversible      = errors.New("non_reversible_rollback_blocked")
)

// RuntimeProvider abstracts the runtime-registry lookup so the orchestrator
// can be wired to either the in-process Store (tests) or the HTTP client.
type RuntimeProvider interface {
	Get(ctx context.Context, runtimeID string) (*rt.Runtime, error)
	CheckUsableBy(ctx context.Context, runtimeID, workspaceID string) error
}

type ImageMetadataResolver interface {
	Resolve(ctx context.Context, image, digest string) (cosign.ImageMetadata, error)
}

// AssetDeploymentRecorder hands completed deployments to the asset registry
// so the application asset's deployment sub-resource is updated. Production
// wires this to the registry HTTP API; tests use an in-memory recorder.
type AssetDeploymentRecorder interface {
	Record(ctx context.Context, d *Deployment) error
}

type ApprovalsInbox interface {
	CreateDeploymentApproval(ctx context.Context, item ApprovalInboxItem) error
}

type ApprovalInboxItem struct {
	DeploymentID        string
	WorkspaceID         string
	AssetID             string
	Env                 string
	RuntimeID           string
	ImageDigest         string
	RevisionID          string
	RollbackPlanSummary string
	RiskSummary         string
	OpenSpecIDs         []string
}

type Service struct {
	Store                 *Store
	Deployers             *deployers.Registry
	Cosign                *cosign.Verifier
	ImageMetadata         ImageMetadataResolver
	Runtimes              RuntimeProvider
	Approvals             ApprovalProvider
	ApprovalsInbox        ApprovalsInbox
	PolicyEngine          ExternalPolicyEvaluator
	Policies              []DeploymentPolicy
	FreezeWindows         []FreezeWindow
	AutoRollbackByEnv     map[string]bool
	OverrideAllowUnsigned func(workspaceID, deploymentID string) (bool, time.Time)
	AssetRecorder         AssetDeploymentRecorder
	Sink                  Sink
	Now                   func() time.Time
}

func NewService(store *Store, registry *deployers.Registry, sink Sink) *Service {
	return &Service{
		Store:     store,
		Deployers: registry,
		Cosign:    cosign.NewVerifier(nil, nil),
		Policies:  DefaultDeploymentPolicies(),
		Sink:      sink,
		Now:       func() time.Time { return time.Now().UTC() },
		Approvals: NewInMemoryApprovals(),
	}
}

// Deploy is the main orchestration entrypoint per the
// `deploy-orchestrator` spec. Stages: requested → preflight → policy →
// image-verify → render → apply → verify → notify.
func (s *Service) Deploy(ctx context.Context, req *DeployRequest) (*DeployResponse, error) {
	if req.RuntimeID == "" {
		return nil, ErrRuntimeRequired
	}
	if req.RequestID == "" {
		req.RequestID = newID()
	}
	now := s.Now()
	d := &Deployment{
		RequestID:          req.RequestID,
		WorkspaceID:        req.WorkspaceID,
		TenantID:           req.TenantID,
		AssetID:            req.AssetID,
		Env:                req.Env,
		Criticality:        req.Criticality,
		DataClassification: req.DataClassification,
		RuntimeID:          req.RuntimeID,
		Image:              req.Image,
		ImageDigest:        req.ImageDigest,
		RevisionID:         newID(),
		OpenSpecIDs:        req.OpenSpecIDs,
		PRSHA:              req.PRSHA,
		Strategy:           defaultStrategy(req.Strategy),
		CanaryPercent:      req.CanaryPercent,
		RollbackPlan:       req.RollbackPlan,
		AutoRollback:       req.AutoRollback || s.AutoRollbackByEnv[req.Env],
		Status:             StatusPending,
		Actor:              req.Actor,
		CreatedAt:          now,
	}
	d, created := s.Store.Insert(d)
	if !created {
		// Idempotent: return existing state.
		return &DeployResponse{Deployment: d, Status: string(d.Status), Reason: d.StatusReason, Created: false}, nil
	}
	s.emitStageEvent(d, StageRequested, OutcomeStarted, "", nil, now, now)
	_ = s.Sink.Emit(newDeploymentEvent(d, "deployment.requested.v1", map[string]any{
		"deployment_id": d.ID, "asset_id": d.AssetID, "env": d.Env, "image": d.Image, "request_id": d.RequestID,
	}))

	// Stage: preflight (runtime lookup + tenancy + connector preflight).
	runtime, err := s.runtimePreflight(ctx, d)
	if err != nil {
		return s.fail(ctx, d, StagePreflight, err)
	}
	connector, err := s.Deployers.For(runtime.Type)
	if err != nil {
		return s.fail(ctx, d, StagePreflight, fmt.Errorf("%w: %s", ErrDeployerMissing, err))
	}
	preCheck := connector.Preflight(ctx, runtime)
	if !preCheck.Passed {
		return s.fail(ctx, d, StagePreflight, fmt.Errorf("preflight failed: %s", preCheck.Reason))
	}
	s.emitStageEvent(d, StagePreflight, OutcomeCompleted, "", map[string]any{"runtime_id": runtime.ID}, s.Now(), s.Now())

	// Stage: policy
	if decision, all, ok := s.runPolicies(ctx, d, req); !ok {
		return s.policyOutcome(ctx, d, decision, all)
	}

	// Stage: image-verify
	if err := s.runImageVerify(ctx, d, req); err != nil {
		return s.fail(ctx, d, StageImageVerify, err)
	}

	// Stage: render
	mfs := req.Manifest
	if mfs.AppName == "" {
		mfs.AppName = d.AssetID
	}
	if mfs.Image == "" {
		mfs.Image = d.Image
	}
	mfs.ImageDigest = d.ImageDigest
	params := deployers.Params{
		Strategy:       deployers.Strategy(d.Strategy),
		CanaryPercent:  d.CanaryPercent,
		RollbackPlan:   d.RollbackPlan,
		OpenSpecIDs:    d.OpenSpecIDs,
		PRSHA:          d.PRSHA,
		RevisionID:     d.RevisionID,
		HealthTimeout:  5 * time.Minute,
		RolloutTimeout: 10 * time.Minute,
	}
	stageStart := s.Now()
	art, err := connector.Render(ctx, mfs, params)
	if err != nil {
		return s.fail(ctx, d, StageRender, err)
	}
	s.Store.SetManifestSHA(d.ID, art.ManifestSHA)
	s.emitStageEvent(d, StageRender, OutcomeCompleted, "", map[string]any{"manifest_sha": art.ManifestSHA}, stageStart, s.Now())

	// Stage: apply
	stageStart = s.Now()
	applyRes, err := connector.Apply(ctx, runtime, art, params)
	if err != nil || applyRes.Outcome != "ok" {
		s.fail(ctx, d, StageApply, fmtErr(err, "apply_failed"))
		return &DeployResponse{Deployment: mustGet(s.Store, d.ID), Status: string(StatusFailed), Reason: "apply_failed"}, nil
	}
	s.emitStageEvent(d, StageApply, OutcomeCompleted, "", map[string]any{"revision_id": d.RevisionID}, stageStart, s.Now())
	_ = s.Sink.Emit(newDeploymentEvent(d, "deployment.applied.v1", map[string]any{
		"deployment_id": d.ID, "revision_id": d.RevisionID, "manifest_sha": art.ManifestSHA,
	}))

	// Stage: verify
	stageStart = s.Now()
	verifyRes, err := connector.Verify(ctx, runtime, mfs, params)
	if err != nil || !verifyRes.Healthy {
		s.emitStageEvent(d, StageVerify, OutcomeFailed, verifyRes.FailReason, verifyRes.Detail, stageStart, s.Now())
		_ = s.Sink.Emit(newDeploymentEvent(d, "deployment.failed.v1", map[string]any{
			"deployment_id": d.ID, "stage": "verify", "reason": verifyRes.FailReason,
		}))
		s.Store.SetStatus(d.ID, StatusFailed, "verify_failed:"+verifyRes.FailReason)
		// Auto-rollback if enabled for the env or per-deployment flag.
		if d.AutoRollback {
			if rb, rbErr := s.rollbackInternal(ctx, d, RollbackRequest{Reason: "auto:verify_failed", Approved: true, Manual: false}); rbErr == nil {
				return &DeployResponse{Deployment: rb.Deployment, Status: rb.Status, Reason: "rolled_back_after_verify_failure"}, nil
			}
		}
		return &DeployResponse{Deployment: mustGet(s.Store, d.ID), Status: string(StatusFailed), Reason: verifyRes.FailReason, Created: created}, nil
	}
	s.emitStageEvent(d, StageVerify, OutcomeCompleted, "", verifyRes.Detail, stageStart, s.Now())
	_ = s.Sink.Emit(newDeploymentEvent(d, "deployment.verified.v1", map[string]any{
		"deployment_id": d.ID, "revision_id": d.RevisionID,
	}))

	// Stage: notify
	s.emitStageEvent(d, StageNotify, OutcomeCompleted, "", nil, s.Now(), s.Now())
	s.Store.SetStatus(d.ID, StatusCompleted, "")
	final := mustGet(s.Store, d.ID)
	if s.AssetRecorder != nil {
		_ = s.AssetRecorder.Record(ctx, final)
	}
	return &DeployResponse{Deployment: final, Status: string(StatusCompleted), Created: created}, nil
}

// Trigger is a tiny helper string type to avoid struct-literal noise.
type Trigger string

func (t Trigger) String() string { return string(t) }

func (s *Service) runtimePreflight(ctx context.Context, d *Deployment) (*rt.Runtime, error) {
	if s.Runtimes == nil {
		return nil, errors.New("runtime provider not configured")
	}
	if err := s.Runtimes.CheckUsableBy(ctx, d.RuntimeID, d.WorkspaceID); err != nil {
		return nil, err
	}
	rtm, err := s.Runtimes.Get(ctx, d.RuntimeID)
	if err != nil {
		return nil, err
	}
	if rtm == nil {
		return nil, ErrRuntimeNotFound
	}
	return rtm, nil
}

func (s *Service) runPolicies(ctx context.Context, d *Deployment, req *DeployRequest) (PolicyDecision, []PolicyDecision, bool) {
	stageStart := s.Now()
	pctx := PolicyContext{
		Now:           stageStart,
		FreezeWindows: s.FreezeWindows,
	}
	if s.Approvals != nil {
		hasApproval, exp, revID, _ := s.Approvals.HasApproval(d.WorkspaceID, d.AssetID, d.Env, d.ImageDigest)
		pctx.HasApproval = hasApproval
		pctx.ApprovalExpiresAt = exp
		pctx.ApprovalRevisionID = revID
	}
	if s.OverrideAllowUnsigned != nil {
		active, ttl := s.OverrideAllowUnsigned(d.WorkspaceID, d.ID)
		pctx.OverrideAllowUnsignedActive = active
		pctx.OverrideAllowUnsignedTTL = ttl
	}
	decision, all := EvaluatePolicies(s.Policies, req, pctx)
	if s.PolicyEngine != nil {
		if external, err := s.PolicyEngine.Evaluate(ctx, req); err != nil {
			all = append(all, PolicyDecision{PolicyID: "policy-engine", Decision: DecisionDeny, Reason: "policy_engine_unavailable", Rationale: err.Error()})
			decision = MostRestrictivePolicyDecision(all)
		} else {
			all = append([]PolicyDecision{external}, all...)
			decision = MostRestrictivePolicyDecision(all)
		}
	}
	for _, eval := range all {
		s.Store.AppendPolicyEval(PolicyEvalResult{
			DeploymentID: d.ID, PolicyID: eval.PolicyID,
			Outcome: string(eval.Decision), Reason: eval.Reason,
			Detail: map[string]any{"rationale": eval.Rationale, "required_approvers": eval.RequiredApprovers},
		})
	}
	_ = s.Sink.Emit(newDeploymentEvent(d, "deployment.policy_evaluated.v1", map[string]any{
		"deployment_id": d.ID,
		"outcome":       string(decision.Decision),
		"reason":        decision.Reason,
		"rationale":     decision.Rationale,
		"evaluations":   all,
	}))
	if decision.Decision == DecisionAllow {
		s.emitStageEvent(d, StagePolicy, OutcomeCompleted, "", map[string]any{"policy_id": decision.PolicyID}, stageStart, s.Now())
		return decision, all, true
	}
	return decision, all, false
}

func (s *Service) policyOutcome(ctx context.Context, d *Deployment, decision PolicyDecision, all []PolicyDecision) (*DeployResponse, error) {
	now := s.Now()
	if decision.Decision == DecisionRequiresApproval {
		s.Store.SetStatus(d.ID, StatusPendingApproval, decision.Reason)
		s.emitStageEvent(d, StagePolicy, OutcomeDenied, decision.Reason, map[string]any{"policy_id": decision.PolicyID}, now, now)
		if s.ApprovalsInbox != nil {
			_ = s.ApprovalsInbox.CreateDeploymentApproval(ctx, ApprovalInboxItem{
				DeploymentID:        d.ID,
				WorkspaceID:         d.WorkspaceID,
				AssetID:             d.AssetID,
				Env:                 d.Env,
				RuntimeID:           d.RuntimeID,
				ImageDigest:         d.ImageDigest,
				RevisionID:          d.RevisionID,
				RollbackPlanSummary: d.RollbackPlan,
				OpenSpecIDs:         d.OpenSpecIDs,
				RiskSummary:         decision.Reason,
			})
		}
		return &DeployResponse{Deployment: mustGet(s.Store, d.ID), Status: string(StatusPendingApproval), Reason: decision.Reason}, nil
	}
	// Deny
	s.Store.SetStatus(d.ID, StatusBlocked, decision.Reason)
	s.emitStageEvent(d, StagePolicy, OutcomeDenied, decision.Reason, map[string]any{"policy_id": decision.PolicyID}, now, now)
	return &DeployResponse{Deployment: mustGet(s.Store, d.ID), Status: string(StatusBlocked), Reason: decision.Reason}, nil
}

func (s *Service) runImageVerify(ctx context.Context, d *Deployment, req *DeployRequest) error {
	stageStart := s.Now()
	// Override?
	if s.OverrideAllowUnsigned != nil {
		if active, ttl := s.OverrideAllowUnsigned(d.WorkspaceID, d.ID); active && stageStart.Before(ttl) {
			s.Store.SetVerified(d.ID, false, false)
			s.Store.AppendImageVerification(ImageVerificationResult{
				DeploymentID: d.ID, Outcome: "skipped", Reason: "override_allow_unsigned_active",
				Digest: d.ImageDigest,
			})
			s.emitStageEvent(d, StageImageVerify, OutcomeSkipped, "override_allow_unsigned_active", nil, stageStart, s.Now())
			_ = s.Sink.Emit(newDeploymentEvent(d, "policy.override.consumed.v1", map[string]any{
				"deployment_id": d.ID, "override": "allow-unsigned-image",
			}))
			return nil
		}
	}
	meta := cosign.ImageMetadata{Image: d.Image, Digest: d.ImageDigest}
	if s.ImageMetadata != nil {
		var err error
		meta, err = s.ImageMetadata.Resolve(ctx, d.Image, d.ImageDigest)
		if err != nil {
			return fmt.Errorf("metadata: %w", err)
		}
	}
	res := s.Cosign.Combined(ctx, d.WorkspaceID, meta)
	signature := res.Outcome == cosign.OutcomeSuccess
	s.Store.SetVerified(d.ID, signature, signature)
	s.Store.AppendImageVerification(ImageVerificationResult{
		DeploymentID:        d.ID,
		Outcome:             string(res.Outcome),
		Reason:              string(res.Reason),
		Identity:            res.Identity,
		Digest:              res.Digest,
		SignatureVerified:   signature,
		AttestationVerified: signature,
	})
	if !signature {
		_ = s.Sink.Emit(newDeploymentEvent(d, "deployment.image_verified.v1", map[string]any{
			"deployment_id": d.ID, "outcome": "failed", "reason": string(res.Reason),
		}))
		s.emitStageEvent(d, StageImageVerify, OutcomeFailed, string(res.Reason), nil, stageStart, s.Now())
		return fmt.Errorf("%w: %s", ErrImageVerification, res.Reason)
	}
	_ = s.Sink.Emit(newDeploymentEvent(d, "deployment.image_verified.v1", map[string]any{
		"deployment_id": d.ID, "outcome": "success", "identity": res.Identity, "digest": res.Digest,
	}))
	s.emitStageEvent(d, StageImageVerify, OutcomeCompleted, "", map[string]any{"identity": res.Identity}, stageStart, s.Now())
	return nil
}

// Rollback re-applies the previous revision following the same stage
// discipline (preflight → policy → re-apply → verify) per the
// `deployment-orchestrator` and `deployment-history` specs.
func (s *Service) Rollback(ctx context.Context, deploymentID string, req RollbackRequest) (*DeployResponse, error) {
	d, ok := s.Store.Get(deploymentID)
	if !ok {
		return nil, ErrDeploymentNotFound
	}
	return s.rollbackInternal(ctx, d, req)
}

func (s *Service) rollbackInternal(ctx context.Context, d *Deployment, req RollbackRequest) (*DeployResponse, error) {
	prev := s.Store.PreviousRevision(d.AssetID, d.Env, d.ID)
	if prev == nil {
		return nil, ErrPreviousRevision
	}
	// Block non-reversible (DB-migration) rollbacks unless explicit override.
	if isHighCriticality(d.Criticality) && !req.Approved && strings.Contains(strings.ToLower(d.RollbackPlan), "non-reversible") {
		return nil, ErrNonReversible
	}
	rb := RollbackRecord{
		DeploymentID: d.ID, SourceRevID: d.RevisionID, RestoredRevID: prev.RevisionID,
		Reason: req.Reason, Trigger: triggerOrManual(req.Manual), Approved: req.Approved,
	}
	s.Store.AppendRollback(rb)
	rollbackDep := &Deployment{
		RequestID:          newID(),
		WorkspaceID:        d.WorkspaceID,
		TenantID:           d.TenantID,
		AssetID:            d.AssetID,
		Env:                d.Env,
		Criticality:        d.Criticality,
		DataClassification: d.DataClassification,
		RuntimeID:          d.RuntimeID,
		Image:              prev.Image,
		ImageDigest:        prev.ImageDigest,
		RevisionID:         newID(),
		SourceRevisionID:   prev.RevisionID,
		RollbackOf:         d.ID,
		OpenSpecIDs:        d.OpenSpecIDs,
		Strategy:           d.Strategy,
		Status:             StatusRunning,
		Actor:              req.Actor,
		CorrelationID:      d.CorrelationID,
	}
	rollbackDep, _ = s.Store.Insert(rollbackDep)
	now := s.Now()
	s.emitStageEvent(rollbackDep, StageRollback, OutcomeStarted, req.Reason, map[string]any{"rollback_of": d.ID, "restored_rev": prev.RevisionID}, now, now)
	runtime, err := s.runtimePreflight(ctx, rollbackDep)
	if err != nil {
		return s.fail(ctx, rollbackDep, StageRollback, err)
	}
	connector, err := s.Deployers.For(runtime.Type)
	if err != nil {
		return s.fail(ctx, rollbackDep, StageRollback, err)
	}
	prevManifest := deployers.Manifest{AppName: prev.AssetID, Image: prev.Image, ImageDigest: prev.ImageDigest}
	params := deployers.Params{Strategy: deployers.Strategy(rollbackDep.Strategy), PrevRevisionID: prev.RevisionID, RevisionID: rollbackDep.RevisionID}
	res, err := connector.Rollback(ctx, runtime, prevManifest, params)
	if err != nil || res.Outcome != "ok" {
		return s.fail(ctx, rollbackDep, StageRollback, fmtErr(err, "rollback_failed"))
	}
	s.emitStageEvent(rollbackDep, StageRollback, OutcomeCompleted, "", map[string]any{"restored_revision_id": prev.RevisionID}, now, s.Now())
	s.Store.SetStatus(d.ID, StatusRolledBack, req.Reason)
	s.Store.SetStatus(rollbackDep.ID, StatusCompleted, "rollback_complete")
	_ = s.Sink.Emit(newDeploymentEvent(rollbackDep, "deployment.rolled_back.v1", map[string]any{
		"deployment_id":        rollbackDep.ID,
		"rolled_back_id":       d.ID,
		"restored_revision_id": prev.RevisionID,
		"reason":               req.Reason,
		"trigger":              triggerOrManual(req.Manual),
	}))
	if s.AssetRecorder != nil {
		_ = s.AssetRecorder.Record(ctx, mustGet(s.Store, rollbackDep.ID))
	}
	return &DeployResponse{Deployment: mustGet(s.Store, rollbackDep.ID), Status: string(StatusCompleted)}, nil
}

func triggerOrManual(manual bool) string {
	if manual {
		return "manual"
	}
	return "auto"
}

func (s *Service) emitStageEvent(d *Deployment, stage Stage, outcome StageOutcome, reason string, detail map[string]any, start, end time.Time) {
	dur := end.Sub(start)
	s.Store.AppendEvent(DeploymentEvent{
		DeploymentID: d.ID, Stage: stage, Outcome: outcome,
		Reason: reason, Detail: detail,
		StartedAt: start, EndedAt: end, DurationMS: dur.Milliseconds(),
	})
}

func (s *Service) fail(ctx context.Context, d *Deployment, stage Stage, err error) (*DeployResponse, error) {
	now := s.Now()
	s.Store.SetStatus(d.ID, StatusFailed, fmt.Sprintf("%s:%s", stage, err.Error()))
	s.emitStageEvent(d, stage, OutcomeFailed, err.Error(), nil, now, now)
	_ = s.Sink.Emit(newDeploymentEvent(d, "deployment.failed.v1", map[string]any{
		"deployment_id": d.ID, "stage": string(stage), "reason": err.Error(),
	}))
	return &DeployResponse{Deployment: mustGet(s.Store, d.ID), Status: string(StatusFailed), Reason: err.Error()}, nil
}

func mustGet(store *Store, id string) *Deployment {
	d, _ := store.Get(id)
	return d
}

func fmtErr(err error, fallback string) error {
	if err != nil {
		return err
	}
	return errors.New(fallback)
}

func defaultStrategy(s deployers.Strategy) deployers.Strategy {
	if s == "" {
		return deployers.StrategyRolling
	}
	return s
}
