package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SignalSource identifies the type of signal that triggered detection.
type SignalSource string

const (
	SignalPrometheus      SignalSource = "prometheus"
	SignalCIFailed        SignalSource = "ci_failed"
	SignalDeployFailed    SignalSource = "deploy_failed"
	SignalAlertThreshold  SignalSource = "alert_threshold"
	SignalIncidentCreated SignalSource = "incident_created"
)

// DetectionInput is the input to the L1 Detect pipeline.
type DetectionInput struct {
	AppID                string         `json:"app_id"`
	TenantID             string         `json:"tenant_id"`
	WorkspaceID          string         `json:"workspace_id"`
	CorrelationID        string         `json:"correlation_id"`
	SignalSource         SignalSource    `json:"signal_source"`
	SignalPayload        map[string]any `json:"signal_payload"`
	CandidateHypotheses  []Hypothesis   `json:"candidate_hypotheses"`
	CandidateActions     []string       `json:"candidate_actions"`
	BlastRadiusEstimate  string         `json:"blast_radius_estimate"`
}

// Hypothesis is a candidate explanation for an incident.
type Hypothesis struct {
	ID            string   `json:"id"`
	Description   string   `json:"description"`
	Confidence    float64  `json:"confidence"`
	Evidence      []string `json:"evidence"`
	AffectedFiles []string `json:"affected_files"`
}

// DiagnosisReport is the output of the L1 diagnosis step.
type DiagnosisReport struct {
	AppID               string      `json:"app_id"`
	SignalSource        SignalSource `json:"signal_source"`
	Hypotheses          []Hypothesis `json:"hypotheses"`
	CandidateActions    []string    `json:"candidate_actions"`
	BlastRadiusEstimate string      `json:"blast_radius_estimate"`
	GeneratedAt         time.Time   `json:"generated_at"`
}

// ProposeFixInput is the input to the L2 ProposeFix pipeline.
type ProposeFixInput struct {
	AppID            string     `json:"app_id"`
	TenantID         string     `json:"tenant_id"`
	WorkspaceID      string     `json:"workspace_id"`
	CorrelationID    string     `json:"correlation_id"`
	TopHypothesis    Hypothesis `json:"top_hypothesis"`
	DiffType         string     `json:"diff_type"` // "code" | "config"
	ProtectedPaths   []string   `json:"protected_paths"`
	SizeBudgetLines  int        `json:"size_budget_lines"` // default 200
}

// ProposedFix is a candidate code/config change produced by the L2 pipeline.
type ProposedFix struct {
	ID                  string     `json:"id"`
	AppID               string     `json:"app_id"`
	DiffType            string     `json:"diff_type"`
	FileDiffs           []FileDiff `json:"file_diffs"`
	Citations           []string   `json:"citations"`
	BlastRadiusEstimate string     `json:"blast_radius_estimate"`
}

// FileDiff represents a single-file patch.
type FileDiff struct {
	Path         string `json:"path"`
	Before       string `json:"before"`
	After        string `json:"after"`
	LinesChanged int    `json:"lines_changed"`
}

// SafetyEvalResult captures whether a proposed fix passed the safety gate.
type SafetyEvalResult struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

// L1Detection is produced by the Detect method and stored in the Store.
type L1Detection struct {
	ID            string          `json:"id"`
	AppID         string          `json:"app_id"`
	TenantID      string          `json:"tenant_id"`
	CorrelationID string          `json:"correlation_id"`
	Signal        SignalSource    `json:"signal"`
	Diagnosis     DiagnosisReport `json:"diagnosis"`
	CreatedAt     time.Time       `json:"created_at"`
}

// L2Suggestion is produced by the ProposeFix method and stored in the Store.
type L2Suggestion struct {
	ID               string      `json:"id"`
	AppID            string      `json:"app_id"`
	TenantID         string      `json:"tenant_id"`
	CorrelationID    string      `json:"correlation_id"`
	Detection        L1Detection `json:"detection"`
	Fix              ProposedFix `json:"fix"`
	SafetyPassed     bool        `json:"safety_passed"`
	ApprovalID       string      `json:"approval_id,omitempty"`
	Status           string      `json:"status"` // "pending" | "approved" | "rejected"
	RejectionReason  string      `json:"rejection_reason,omitempty"`
	CreatedAt        time.Time   `json:"created_at"`
}

// secretPatterns is checked against every diff hunk in the safety evaluator.
var secretPatterns = []string{
	"password=",
	"secret=",
	"api_key=",
	"token=",
}

// Detect runs the L1 detection pipeline.
// It records a DiagnosisReport built from the caller-supplied hypotheses,
// emits a healing.detected.v1 CloudEvent, persists the detection, and
// creates an approval inbox card.
func (s *Service) Detect(ctx context.Context, in DetectionInput) (*L1Detection, error) {
	if in.AppID == "" {
		return nil, fmt.Errorf("app_id required")
	}

	// Run diagnosis — stubbed: use the caller-supplied hypotheses ranked by
	// confidence descending (a real implementation would call an LLM).
	diagnosis := DiagnosisReport{
		AppID:               in.AppID,
		SignalSource:        in.SignalSource,
		Hypotheses:          in.CandidateHypotheses,
		CandidateActions:    in.CandidateActions,
		BlastRadiusEstimate: in.BlastRadiusEstimate,
		GeneratedAt:         s.Now(),
	}

	detection := &L1Detection{
		ID:            "det-" + uuid.NewString(),
		AppID:         in.AppID,
		TenantID:      in.TenantID,
		CorrelationID: in.CorrelationID,
		Signal:        in.SignalSource,
		Diagnosis:     diagnosis,
		CreatedAt:     s.Now(),
	}

	// Emit CloudEvent.
	_ = s.Sink.Emit(newEvent(in.TenantID, in.WorkspaceID, "healing.detected.v1",
		"app/"+in.AppID, map[string]any{
			"detection_id":          detection.ID,
			"app_id":                in.AppID,
			"signal_source":         in.SignalSource,
			"hypothesis_count":      len(diagnosis.Hypotheses),
			"blast_radius_estimate": diagnosis.BlastRadiusEstimate,
			"correlation_id":        in.CorrelationID,
		}))

	// Persist.
	s.Store.SaveDetection(detection)

	// Create approval inbox card for L1 notification.
	_, _ = s.Approvals.Create(ctx, map[string]any{
		"tag":          "healing-l1",
		"detection_id": detection.ID,
		"app_id":       in.AppID,
		"signal":       in.SignalSource,
		"hypotheses":   len(diagnosis.Hypotheses),
	})

	return detection, nil
}

// ProposeFix runs the L2 propose-fix pipeline.
// It synthesises a ProposedFix from the top hypothesis, runs SafetyEval,
// downgrades to L1 if the fix is unsafe, and otherwise emits
// healing.fix_proposed.v1 and creates an approval card.
func (s *Service) ProposeFix(ctx context.Context, in ProposeFixInput) (*L2Suggestion, error) {
	if in.AppID == "" {
		return nil, fmt.Errorf("app_id required")
	}

	sizeBudget := in.SizeBudgetLines
	if sizeBudget <= 0 {
		sizeBudget = 200
	}

	diffType := in.DiffType
	if diffType == "" {
		diffType = "code"
	}

	// Stub: synthesise a representative diff from the hypothesis evidence.
	fix := s.synthesiseFix(in, diffType)

	// Safety gate.
	evalResult := s.SafetyEval(fix, in.ProtectedPaths, sizeBudget)

	// Build the detection context required by L2Suggestion.
	detection := L1Detection{
		ID:            "det-" + uuid.NewString(),
		AppID:         in.AppID,
		TenantID:      in.TenantID,
		CorrelationID: in.CorrelationID,
		Signal:        SignalIncidentCreated,
		Diagnosis: DiagnosisReport{
			AppID:        in.AppID,
			SignalSource: SignalIncidentCreated,
			Hypotheses:   []Hypothesis{in.TopHypothesis},
			GeneratedAt:  s.Now(),
		},
		CreatedAt: s.Now(),
	}

	suggestion := &L2Suggestion{
		ID:            "sug-" + uuid.NewString(),
		AppID:         in.AppID,
		TenantID:      in.TenantID,
		CorrelationID: in.CorrelationID,
		Detection:     detection,
		Fix:           fix,
		SafetyPassed:  evalResult.Passed,
		Status:        "pending",
		CreatedAt:     s.Now(),
	}

	if !evalResult.Passed {
		// Downgrade: emit fix_downgraded and return without creating an approval card.
		_ = s.Sink.Emit(newEvent(in.TenantID, in.WorkspaceID, "healing.fix_downgraded.v1",
			"app/"+in.AppID, map[string]any{
				"suggestion_id":  suggestion.ID,
				"app_id":         in.AppID,
				"correlation_id": in.CorrelationID,
				"reason":         evalResult.Reason,
			}))
		suggestion.Status = "rejected"
		suggestion.RejectionReason = evalResult.Reason
		s.Store.SaveSuggestion(suggestion)
		return suggestion, nil
	}

	// Safety passed — emit and create approval card.
	_ = s.Sink.Emit(newEvent(in.TenantID, in.WorkspaceID, "healing.fix_proposed.v1",
		"app/"+in.AppID, map[string]any{
			"suggestion_id":         suggestion.ID,
			"app_id":                in.AppID,
			"diff_type":             fix.DiffType,
			"files_changed":         len(fix.FileDiffs),
			"blast_radius_estimate": fix.BlastRadiusEstimate,
			"correlation_id":        in.CorrelationID,
		}))

	approvalID, _ := s.Approvals.Create(ctx, map[string]any{
		"tag":           "healing-l2",
		"suggestion_id": suggestion.ID,
		"app_id":        in.AppID,
		"diff_type":     fix.DiffType,
	})
	suggestion.ApprovalID = approvalID

	s.Store.SaveSuggestion(suggestion)
	return suggestion, nil
}

// synthesiseFix builds a stub ProposedFix from the hypothesis.
// In a production implementation this would call an LLM code-gen endpoint.
func (s *Service) synthesiseFix(in ProposeFixInput, diffType string) ProposedFix {
	var diffs []FileDiff
	for _, f := range in.TopHypothesis.AffectedFiles {
		diffs = append(diffs, FileDiff{
			Path:         f,
			Before:       "# original content",
			After:        "# patched by healing-engine: " + in.TopHypothesis.Description,
			LinesChanged: 2,
		})
	}
	if len(diffs) == 0 {
		// Fallback placeholder diff so the fix is always non-empty.
		diffs = []FileDiff{{
			Path:         "config/remediation.yaml",
			Before:       "",
			After:        "# auto-remediation placeholder",
			LinesChanged: 1,
		}}
	}
	totalLines := 0
	for _, d := range diffs {
		totalLines += d.LinesChanged
	}
	_ = totalLines // used implicitly via diffs

	return ProposedFix{
		ID:                  "fix-" + uuid.NewString(),
		AppID:               in.AppID,
		DiffType:            diffType,
		FileDiffs:           diffs,
		Citations:           in.TopHypothesis.Evidence,
		BlastRadiusEstimate: "low",
	}
}

// SafetyEval checks that a ProposedFix satisfies the three safety criteria:
//  1. Total lines changed <= sizeBudget.
//  2. No file path matches any entry in protectedPaths (substring match).
//  3. No diff content contains known secret reference patterns.
func (s *Service) SafetyEval(fix ProposedFix, protectedPaths []string, sizeBudget int) SafetyEvalResult {
	// 1. Size budget.
	total := 0
	for _, d := range fix.FileDiffs {
		total += d.LinesChanged
	}
	if total > sizeBudget {
		return SafetyEvalResult{
			Passed: false,
			Reason: fmt.Sprintf("size_budget_exceeded: %d lines changed, budget is %d", total, sizeBudget),
		}
	}

	// 2. Protected paths.
	for _, d := range fix.FileDiffs {
		for _, pp := range protectedPaths {
			if strings.Contains(d.Path, pp) {
				return SafetyEvalResult{
					Passed: false,
					Reason: fmt.Sprintf("protected_path_violation: %q matches protected pattern %q", d.Path, pp),
				}
			}
		}
	}

	// 3. Secret references in diff content.
	for _, d := range fix.FileDiffs {
		combined := strings.ToLower(d.Before + "\n" + d.After)
		for _, pat := range secretPatterns {
			if strings.Contains(combined, pat) {
				return SafetyEvalResult{
					Passed: false,
					Reason: fmt.Sprintf("secret_reference_detected: pattern %q found in diff for %q", pat, d.Path),
				}
			}
		}
	}

	return SafetyEvalResult{Passed: true}
}
