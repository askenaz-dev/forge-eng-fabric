package sdlc

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type GateDefinition struct {
	Name           string
	MinCriticality string
}

type GateEvaluator interface {
	Evaluate(ctx context.Context, initiative *Initiative, phase Phase, evidence map[string]any) ([]GateResult, error)
}

type EvidenceGateEvaluator struct {
	Now func() time.Time
}

func (e EvidenceGateEvaluator) Evaluate(_ context.Context, initiative *Initiative, phase Phase, evidence map[string]any) ([]GateResult, error) {
	defs := RequiredGates(phase, initiative.Criticality)
	now := time.Now().UTC()
	if e.Now != nil {
		now = e.Now().UTC()
	}
	results := make([]GateResult, 0, len(defs))
	for _, def := range defs {
		outcome, reason, detail := evaluateEvidence(def.Name, evidence[def.Name])
		results = append(results, GateResult{
			ID:           newID(),
			InitiativeID: initiative.ID,
			Phase:        phase,
			Gate:         def.Name,
			Outcome:      outcome,
			Reason:       reason,
			EvaluatedAt:  now,
			Detail:       detail,
		})
	}
	return results, nil
}

func RequiredGates(phase Phase, criticality string) []GateDefinition {
	all := map[Phase][]GateDefinition{
		PhaseProduct: {
			{Name: "acceptance_criteria_present"},
			{Name: "story_size_estimated"},
		},
		PhaseArchitecture: {
			{Name: "adrs_published"},
			{Name: "api_contract_published"},
			{Name: "data_model_documented"},
			{Name: "threat_model_present", MinCriticality: "medium"},
			{Name: "security_review_passed"},
			{Name: "openspec_updated"},
		},
		PhaseDesign: {
			{Name: "ui_blueprint_present"},
			{Name: "component_stubs_committed"},
			{Name: "accessibility_audit_passed"},
		},
		PhaseDevelopment: {
			{Name: "code_complete"},
			{Name: "lint_clean"},
			{Name: "unit_tests_passing"},
			{Name: "coverage"},
		},
		PhaseQA: {
			{Name: "integration_tests_passing"},
			{Name: "e2e_tests_passing"},
			{Name: "perf_budget_met", MinCriticality: "high"},
		},
		PhaseSecurity: {
			{Name: "sast_clean"},
			{Name: "sca_clean"},
			{Name: "dast_passed", MinCriticality: "high"},
			{Name: "secrets_clean"},
		},
		PhaseDevOps: {
			{Name: "pipelines_green"},
			{Name: "image_signed"},
			{Name: "deploy_to_stage_successful"},
			{Name: "rollback_plan_present"},
		},
		PhaseInfrastructure: {
			{Name: "iac_generated"},
			{Name: "iac_validated"},
			{Name: "iac_applied"},
		},
		PhaseSRE: {
			{Name: "slos_defined"},
			{Name: "runbook_published"},
			{Name: "alerts_configured"},
			{Name: "on_call_assigned"},
		},
		PhaseFinOps: {
			{Name: "cost_estimate_within_budget"},
			{Name: "llm_budget_within_limit"},
		},
		PhaseObservability: {
			{Name: "dashboards_provisioned"},
			{Name: "log_pipeline_active"},
			{Name: "tracing_enabled"},
		},
	}
	out := []GateDefinition{}
	for _, def := range all[phase] {
		if def.MinCriticality == "" || criticalityAtLeast(criticality, def.MinCriticality) {
			out = append(out, def)
		}
	}
	return out
}

func evaluateEvidence(gate string, raw any) (GateOutcome, string, map[string]any) {
	switch value := raw.(type) {
	case bool:
		if value {
			return GatePassed, "", map[string]any{"source": "evidence"}
		}
		return GateFailed, fmt.Sprintf("%s_failed", gate), map[string]any{"source": "evidence"}
	case string:
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "passed" || normalized == "pass" || normalized == "true" || normalized == "ok" {
			return GatePassed, "", map[string]any{"source": "evidence", "value": value}
		}
		return GateFailed, value, map[string]any{"source": "evidence", "value": value}
	case map[string]any:
		outcome := strings.ToLower(fmt.Sprint(value["outcome"]))
		if outcome == "passed" || outcome == "pass" || outcome == "true" || outcome == "ok" {
			return GatePassed, "", value
		}
		reason := fmt.Sprint(value["reason"])
		if reason == "" || reason == "<nil>" {
			reason = fmt.Sprintf("%s_failed", gate)
		}
		return GateFailed, reason, value
	default:
		return GateFailed, "gate_evidence_missing", map[string]any{"source": "evidence"}
	}
}

func criticalityAtLeast(value, minimum string) bool {
	rank := map[string]int{"low": 1, "medium": 2, "high": 3, "critical": 4}
	v := rank[strings.ToLower(value)]
	if v == 0 {
		v = rank["medium"]
	}
	return v >= rank[minimum]
}
