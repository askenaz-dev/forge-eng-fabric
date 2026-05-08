package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"
)

// ApprovalsInboxClient creates entries in the platform Approvals Inbox.
//
// Two implementations are provided:
//   - HTTPApprovalsClient: production wiring against the approvals service.
//   - InMemoryApprovalsClient: used by tests; deliveries arrive via signal.
type ApprovalsInboxClient interface {
	CreateEntry(ctx context.Context, entry ApprovalEntry) (string, error)
}

// ApprovalEntry is the data sent to the Approvals Inbox.
type ApprovalEntry struct {
	WorkflowID      string         `json:"workflow_id"`
	WorkflowVersion string         `json:"workflow_version"`
	ExecutionID     string         `json:"execution_id"`
	StepID          string         `json:"step_id"`
	TenantID        string         `json:"tenant_id"`
	WorkspaceID     string         `json:"workspace_id"`
	ApproverRole    string         `json:"approver_role"`
	OnTimeout       string         `json:"on_timeout"`
	Inputs          map[string]any `json:"inputs"`
	ExpectedOutputs map[string]any `json:"expected_outputs,omitempty"`
	PreviousSteps   []string       `json:"previous_steps,omitempty"`
	NextSteps       []string       `json:"next_steps,omitempty"`
}

// HITLActivity is the production HITL activity. It posts to the Approvals
// Inbox, blocks awaiting an `approve`/`reject` signal, captures the diff
// between proposed and approved inputs, and emits audit events.
type HITLActivity struct {
	Approvals ApprovalsInboxClient
	Audit     AuditLogger
}

// AuditLogger records HITL audit events.
type AuditLogger interface {
	Log(ctx context.Context, entry AuditEntry) error
}

// AuditEntry captures the HITL audit fields required by the spec.
type AuditEntry struct {
	ExecutionID     string         `json:"execution_id"`
	StepID          string         `json:"step_id"`
	Approver        string         `json:"approver"`
	ApproverRole    string         `json:"approver_role"`
	Decision        string         `json:"decision"`
	OriginalInputs  map[string]any `json:"original_inputs"`
	FinalInputs     map[string]any `json:"final_inputs"`
	InputDiff       map[string]any `json:"input_diff"`
	At              time.Time      `json:"at"`
	OnTimeoutPolicy string         `json:"on_timeout_policy,omitempty"`
}

// NoopAuditLogger discards entries.
type NoopAuditLogger struct{}

func (NoopAuditLogger) Log(_ context.Context, _ AuditEntry) error { return nil }

// Execute creates the Approvals Inbox entry and returns Wait=true so the
// engine suspends. The signal-resume path is responsible for delivering the
// approved inputs and (in this implementation) recording the audit entry.
func (h *HITLActivity) Execute(ctx context.Context, in ActivityInput) (ActivityOutput, error) {
	entry := ApprovalEntry{
		ExecutionID:  in.ExecutionID,
		StepID:       in.Step.ID,
		TenantID:     in.TenantID,
		WorkspaceID:  in.WorkspaceID,
		ApproverRole: in.Step.Approver,
		OnTimeout:    in.Step.OnTimeout,
		Inputs:       in.Inputs,
	}
	if h.Approvals != nil {
		_, err := h.Approvals.CreateEntry(ctx, entry)
		if err != nil {
			return ActivityOutput{}, fmt.Errorf("approvals_inbox_unavailable: %w", err)
		}
	}
	return ActivityOutput{
		Outputs: map[string]any{"approver_role": in.Step.Approver},
		Wait:    true,
		Reason:  "awaiting_human_approval",
	}, nil
}

// HTTPApprovalsClient posts ApprovalEntry to the approvals service.
type HTTPApprovalsClient struct {
	BaseURL string
	HTTP    *http.Client
}

// CreateEntry POSTs the entry to /v1/inbox.
func (c *HTTPApprovalsClient) CreateEntry(ctx context.Context, entry ApprovalEntry) (string, error) {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 10 * time.Second}
	}
	body, err := json.Marshal(entry)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/inbox", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("approvals_status_%d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.ID, nil
}

// DiffInputs returns the symmetric difference between two input maps.
//
// Used by the engine to record the audit entry when an approver modifies
// inputs before approval.
func DiffInputs(original, final map[string]any) map[string]any {
	diff := map[string]any{}
	for k, v := range original {
		if v2, ok := final[k]; !ok {
			diff[k] = map[string]any{"removed": v}
		} else if !reflect.DeepEqual(v, v2) {
			diff[k] = map[string]any{"from": v, "to": v2}
		}
	}
	for k, v := range final {
		if _, ok := original[k]; !ok {
			diff[k] = map[string]any{"added": v}
		}
	}
	return diff
}
