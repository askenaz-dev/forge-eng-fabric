package trigger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// RuntimeClient is the seam trigger-router uses to start executions in
// workflow-runtime. cmd/main.go wires an HTTP-backed implementation;
// tests wire FakeRuntime.
type RuntimeClient interface {
	StartExecution(ctx context.Context, req StartExecutionRequest) (StartExecutionResponse, error)
}

// StartExecutionRequest is the body trigger-router POSTs to
// workflow-runtime's /v1/executions endpoint. Matches the shape of
// services/workflow-runtime/internal/runtime.StartRequest.
type StartExecutionRequest struct {
	TenantID      string         `json:"tenant_id"`
	WorkspaceID   string         `json:"workspace_id"`
	WorkflowID    string         `json:"workflow_id"`
	Version       string         `json:"version"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	TriggerEvent  *TriggerEvent  `json:"trigger_event,omitempty"`
	Inputs        map[string]any `json:"inputs,omitempty"`
}

// StartExecutionResponse minimally tracks the execution id allocated by
// workflow-runtime.
type StartExecutionResponse struct {
	ExecutionID string `json:"execution_id"`
}

// TriggerEvent is the payload trigger-router attaches to every
// trigger-originated execution. Mirrors workflow-runtime.TriggerEvent.
type TriggerEvent struct {
	TriggerID     string         `json:"trigger_id"`
	FiredAt       time.Time      `json:"fired_at"`
	Payload       map[string]any `json:"payload,omitempty"`
	QueuePosition int            `json:"queue_position,omitempty"`
}

// HTTPRuntimeClient calls workflow-runtime over HTTP. Retries with
// exponential backoff up to MaxAttempts; persistent failures are sent
// to the dead-letter sink (cmd/main.go wires it).
type HTTPRuntimeClient struct {
	BaseURL     string
	HTTP        *http.Client
	MaxAttempts int
}

func (c *HTTPRuntimeClient) StartExecution(ctx context.Context, req StartExecutionRequest) (StartExecutionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return StartExecutionResponse{}, fmt.Errorf("marshal: %w", err)
	}
	attempts := c.MaxAttempts
	if attempts <= 0 {
		attempts = 4
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/executions", bytes.NewReader(body))
		if err != nil {
			return StartExecutionResponse{}, fmt.Errorf("new request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		resp, err := c.HTTP.Do(httpReq)
		if err != nil {
			lastErr = err
			waitBackoff(i)
			continue
		}
		defer resp.Body.Close()
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			var out StartExecutionResponse
			_ = json.NewDecoder(resp.Body).Decode(&out)
			return out, nil
		case resp.StatusCode == http.StatusConflict:
			// 409 = drop-concurrency policy refused; not retryable.
			return StartExecutionResponse{}, ErrDropConcurrency
		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("upstream %d", resp.StatusCode)
			waitBackoff(i)
			continue
		default:
			return StartExecutionResponse{}, fmt.Errorf("upstream %d (non-retryable)", resp.StatusCode)
		}
	}
	if lastErr == nil {
		lastErr = errors.New("dispatch_exhausted")
	}
	return StartExecutionResponse{}, lastErr
}

func waitBackoff(attempt int) {
	d := time.Duration(100*(1<<attempt)) * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	time.Sleep(d)
}

// ErrDropConcurrency signals the runtime refused to start a new
// execution because the trigger's concurrency policy is `drop` and an
// execution is already in flight. Dispatcher converts this into a
// workflow.trigger.dropped.v1 event.
var ErrDropConcurrency = errors.New("drop_concurrency")

// Dispatcher converts a trigger firing into a workflow-runtime execution
// start, with retry, dead-lettering, and observability event emission.
type Dispatcher struct {
	Runtime RuntimeClient
	Sink    EventSink
	DLQ     DeadLetterSink
}

// DeadLetterSink receives subscriptions whose dispatch failed permanently.
// In dev mode it can be NoopDLQ; production wires it to durable storage.
type DeadLetterSink interface {
	Record(sub Subscription, payload map[string]any, err error) error
}

// NoopDLQ discards dead-lettered firings.
type NoopDLQ struct{}

func (NoopDLQ) Record(Subscription, map[string]any, error) error { return nil }

// Fire dispatches an execution and emits the appropriate observability
// event. Returns the execution id on success.
func (d *Dispatcher) Fire(ctx context.Context, sub Subscription, payload map[string]any) (string, error) {
	correlationID := uuid.NewString()
	req := StartExecutionRequest{
		TenantID:      sub.TenantID,
		WorkspaceID:   sub.WorkspaceID,
		WorkflowID:    sub.WorkflowID,
		Version:       sub.Version,
		CorrelationID: correlationID,
		TriggerEvent: &TriggerEvent{
			TriggerID: sub.TriggerID,
			FiredAt:   time.Now(),
			Payload:   payload,
		},
	}
	resp, err := d.Runtime.StartExecution(ctx, req)
	if err != nil {
		if errors.Is(err, ErrDropConcurrency) {
			_ = d.Sink.Emit(newEvent(sub, EventTriggerDropped, correlationID, map[string]any{
				"reason": "drop_concurrency",
			}))
			return "", err
		}
		_ = d.Sink.Emit(newEvent(sub, EventTriggerFailed, correlationID, map[string]any{
			"reason": err.Error(),
		}))
		if d.DLQ != nil {
			_ = d.DLQ.Record(sub, payload, err)
		}
		return "", err
	}
	_ = d.Sink.Emit(newEvent(sub, EventTriggerFired, correlationID, map[string]any{
		"execution_id": resp.ExecutionID,
	}))
	return resp.ExecutionID, nil
}
