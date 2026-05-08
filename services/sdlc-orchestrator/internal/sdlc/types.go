package sdlc

import (
	"time"

	"github.com/google/uuid"
)

type Phase string

const (
	PhaseProduct      Phase = "product"
	PhaseArchitecture Phase = "architecture"
	PhaseDesign       Phase = "design"
	PhaseDevelopment  Phase = "development"
	PhaseQA           Phase = "qa"
	PhaseSecurity     Phase = "security"
	PhaseDevOps       Phase = "devops"
	PhaseSRE          Phase = "sre"
	PhaseFinOps       Phase = "finops"
	PhaseDone         Phase = "done"
)

var OrderedPhases = []Phase{
	PhaseProduct,
	PhaseArchitecture,
	PhaseDesign,
	PhaseDevelopment,
	PhaseQA,
	PhaseSecurity,
	PhaseDevOps,
	PhaseSRE,
	PhaseFinOps,
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
	ID           string       `json:"id"`
	WorkspaceID  string       `json:"workspace_id"`
	OpenSpecRoot string       `json:"openspec_root"`
	JiraEpicKey  string       `json:"jira_epic_key,omitempty"`
	Criticality  string       `json:"criticality"`
	CurrentPhase Phase        `json:"current_phase"`
	PhaseStates  []PhaseState `json:"phase_states"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
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
	WorkspaceID   string `json:"workspace_id"`
	OpenSpecRoot  string `json:"openspec_root"`
	JiraEpicKey   string `json:"jira_epic_key,omitempty"`
	Criticality   string `json:"criticality,omitempty"`
	Actor         string `json:"actor,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
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
