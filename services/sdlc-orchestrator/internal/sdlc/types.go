package sdlc

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrCannotRelaxRequired is returned by MergeTargets when a spec override
// attempts to make a required phase optional or skipped.
var ErrCannotRelaxRequired = errors.New("cannot relax a required phase")

type Phase string

const (
	PhaseProduct        Phase = "product"
	PhaseArchitecture   Phase = "architecture"
	PhaseDesign         Phase = "design"
	PhaseDevelopment    Phase = "development"
	PhaseQA             Phase = "qa"
	PhaseSecurity       Phase = "security"
	PhaseDevOps         Phase = "devops"
	PhaseInfrastructure Phase = "iac"  // sits between DevOps and SRE (sdlc-end-to-end §7.5)
	PhaseSRE            Phase = "sre"
	PhaseFinOps         Phase = "finops"
	PhaseObservability  Phase = "observability"
	PhaseDone           Phase = "done"
)

// OrderedPhases is the canonical phase execution order. PhaseInfrastructure
// sits between PhaseDevOps and PhaseSRE per the sdlc-end-to-end spec.
var OrderedPhases = []Phase{
	PhaseProduct,
	PhaseArchitecture,
	PhaseDesign,
	PhaseDevelopment,
	PhaseQA,
	PhaseSecurity,
	PhaseDevOps,
	PhaseInfrastructure,
	PhaseSRE,
	PhaseFinOps,
	PhaseObservability,
}

// TargetPolicy declares how a phase should behave in the SDLC plan.
type TargetPolicy string

const (
	// TargetRequired: phase runs; workflow fails on gate failure.
	TargetRequired TargetPolicy = "required"
	// TargetOptional: phase runs; gate failure emits warning but does not fail.
	TargetOptional TargetPolicy = "optional"
	// TargetOptIn: phase runs only when explicitly requested at workflow start.
	TargetOptIn TargetPolicy = "opt-in"
	// TargetSkipped: phase removed from the plan entirely.
	TargetSkipped TargetPolicy = "skipped"
)

// AllTargetPolicies is the exhaustive set of allowed values.
var AllTargetPolicies = map[TargetPolicy]struct{}{
	TargetRequired: {},
	TargetOptional: {},
	TargetOptIn:    {},
	TargetSkipped:  {},
}

// DefaultTargets returns the platform default targets — mirrors the App
// entity defaults from the application service.
func DefaultTargets() map[Phase]TargetPolicy {
	return map[Phase]TargetPolicy{
		PhaseArchitecture:   TargetRequired,
		PhaseDesign:         TargetOptional,
		PhaseDevelopment:    TargetRequired,
		PhaseQA:             TargetRequired,
		PhaseSecurity:       TargetRequired,
		PhaseDevOps:         TargetRequired,
		PhaseInfrastructure: TargetOptIn,
		PhaseSRE:            TargetOptional,
		PhaseFinOps:         TargetOptIn,
		PhaseObservability:  TargetOptIn,
	}
}

// MergeTargets merges a per-spec override on top of the App-level targets.
// The override may only tighten, never relax (required cannot become optional/skipped).
// Returns an error satisfying ErrCannotRelaxRequired when the rule is violated.
func MergeTargets(appTargets, override map[Phase]TargetPolicy) (map[Phase]TargetPolicy, error) {
	rank := map[TargetPolicy]int{TargetSkipped: 0, TargetOptIn: 1, TargetOptional: 2, TargetRequired: 3}
	merged := make(map[Phase]TargetPolicy, len(appTargets))
	for k, v := range appTargets {
		merged[k] = v
	}
	for phase, ov := range override {
		current := merged[phase]
		if rank[ov] < rank[current] {
			return nil, fmt.Errorf("%w: phase=%s app=%s override=%s", ErrCannotRelaxRequired, phase, current, ov)
		}
		merged[phase] = ov
	}
	return merged, nil
}

type PhaseStatus string

const (
	StatusNotStarted  PhaseStatus = "not_started"
	StatusInProgress  PhaseStatus = "in_progress"
	StatusGatePending PhaseStatus = "gate_pending"
	StatusPassed      PhaseStatus = "passed"
	StatusFailed      PhaseStatus = "failed"
	StatusSkipped     PhaseStatus = "skipped"
	StatusOverridden  PhaseStatus = "overridden"
	StatusBlocked     PhaseStatus = "blocked"
)

type GateOutcome string

const (
	GatePassed  GateOutcome = "passed"
	GateFailed  GateOutcome = "failed"
	GateSkipped GateOutcome = "skipped"
)

type Initiative struct {
	ID           string              `json:"id"`
	WorkspaceID  string              `json:"workspace_id"`
	OpenSpecRoot string              `json:"openspec_root"`
	JiraEpicKey  string              `json:"jira_epic_key,omitempty"`
	Criticality  string              `json:"criticality"`
	CurrentPhase Phase               `json:"current_phase"`
	PhaseStates  []PhaseState        `json:"phase_states"`
	Targets      map[Phase]TargetPolicy `json:"targets,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

type PhaseState struct {
	InitiativeID string       `json:"initiative_id"`
	Phase        Phase        `json:"phase"`
	Status       PhaseStatus  `json:"status"`
	EnteredAt    *time.Time   `json:"entered_at,omitempty"`
	CompletedAt  *time.Time   `json:"completed_at,omitempty"`
	Gates        []GateResult `json:"gates"`
	Blockers     []Blocker    `json:"blockers"`
}

type GateResult struct {
	ID           string         `json:"id"`
	InitiativeID string         `json:"initiative_id"`
	Phase        Phase          `json:"phase"`
	Gate         string         `json:"gate"`
	Outcome      GateOutcome    `json:"outcome"`
	Reason       string         `json:"reason,omitempty"`
	EvaluatedAt  time.Time      `json:"evaluated_at"`
	Detail       map[string]any `json:"detail,omitempty"`
}

type Blocker struct {
	ID           string     `json:"id"`
	InitiativeID string     `json:"initiative_id"`
	Phase        Phase      `json:"phase"`
	Gate         string     `json:"gate"`
	Reason       string     `json:"reason"`
	CreatedAt    time.Time  `json:"created_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

type CreateInitiativeRequest struct {
	WorkspaceID     string              `json:"workspace_id"`
	OpenSpecRoot    string              `json:"openspec_root"`
	JiraEpicKey     string              `json:"jira_epic_key,omitempty"`
	Criticality     string              `json:"criticality,omitempty"`
	Actor           string              `json:"actor,omitempty"`
	TenantID        string              `json:"tenant_id,omitempty"`
	CorrelationID   string              `json:"correlation_id,omitempty"`
	Targets         map[Phase]TargetPolicy `json:"targets,omitempty"`
	TargetsOverride map[Phase]TargetPolicy `json:"targets_override,omitempty"`
	// Include lists opt-in phase names explicitly enabled for this run.
	Include []Phase `json:"include,omitempty"`
}

type CompletePhaseRequest struct {
	Actor         string         `json:"actor,omitempty"`
	TenantID      string         `json:"tenant_id,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Evidence      map[string]any `json:"evidence,omitempty"`
	Override      *OverrideInput `json:"override,omitempty"`
}

type OverrideInput struct {
	ID           string `json:"id,omitempty"`
	Approved     bool   `json:"approved"`
	ApprovedBy   string `json:"approved_by"`
	ApproverRole string `json:"approver_role"`
	Reason       string `json:"reason"`
	TTLSeconds   int    `json:"ttl_seconds"`
}

type BusEvent struct {
	Type          string         `json:"type"`
	Subject       string         `json:"subject,omitempty"`
	TenantID      string         `json:"tenant_id,omitempty"`
	WorkspaceID   string         `json:"workspace_id,omitempty"`
	Actor         string         `json:"actor,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Data          map[string]any `json:"data"`
}

func newID() string { return uuid.NewString() }

func NextPhase(phase Phase) (Phase, bool) {
	for i, candidate := range OrderedPhases {
		if candidate == phase {
			if i == len(OrderedPhases)-1 {
				return PhaseDone, true
			}
			return OrderedPhases[i+1], true
		}
	}
	return "", false
}

func PhaseIndex(phase Phase) int {
	if phase == PhaseDone {
		return len(OrderedPhases)
	}
	for i, candidate := range OrderedPhases {
		if candidate == phase {
			return i
		}
	}
	return -1
}
