package trigger

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

func sampleWorkflow(triggers ...ast.Trigger) *ast.Workflow {
	return &ast.Workflow{
		APIVersion: ast.APIVersion, Kind: ast.Kind,
		Metadata: ast.Metadata{ID: "wf-1", Name: "wf", Version: "1.0.0"},
		Spec: ast.Spec{
			Triggers: triggers,
			Steps:    []ast.Step{{ID: "a", Type: ast.StepSkill, Ref: "registry:skill/x/y@1.0.0"}},
		},
	}
}

func TestRegistryIngestsTriggers(t *testing.T) {
	reg := NewRegistry()
	reg.IngestWorkflow("t1", "w1", sampleWorkflow(
		ast.Trigger{ID: "cron-1", Type: ast.TriggerCron},
		ast.Trigger{ID: "hook-1", Type: ast.TriggerWebhookIn},
	))
	if got := len(reg.All()); got != 2 {
		t.Errorf("subscriptions: got %d want 2", got)
	}
	if _, ok := reg.Lookup("wf-1", "hook-1", ""); !ok {
		t.Errorf("hook-1 not found")
	}
	if got := len(reg.ByType(ast.TriggerCron)); got != 1 {
		t.Errorf("cron subs: got %d want 1", got)
	}
}

// FakeRuntime captures dispatch calls and lets the test control the response.
type FakeRuntime struct {
	Requests []StartExecutionRequest
	Resp     StartExecutionResponse
	Err      error
}

func (f *FakeRuntime) StartExecution(_ context.Context, req StartExecutionRequest) (StartExecutionResponse, error) {
	f.Requests = append(f.Requests, req)
	return f.Resp, f.Err
}

func TestDispatcherFireEmitsFiredEvent(t *testing.T) {
	sink := &MemorySink{}
	rt := &FakeRuntime{Resp: StartExecutionResponse{ExecutionID: "exec-123"}}
	d := &Dispatcher{Runtime: rt, Sink: sink, DLQ: NoopDLQ{}}
	sub := Subscription{
		TenantID: "t1", WorkspaceID: "w1",
		WorkflowID: "wf-1", Version: "1.0.0",
		TriggerID: "src", Type: ast.TriggerEventBus,
	}
	execID, err := d.Fire(context.Background(), sub, map[string]any{"k": "v"})
	if err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if execID != "exec-123" {
		t.Errorf("execID: got %q want exec-123", execID)
	}
	if len(rt.Requests) != 1 {
		t.Errorf("expected one runtime call, got %d", len(rt.Requests))
	}
	if rt.Requests[0].TriggerEvent == nil || rt.Requests[0].TriggerEvent.TriggerID != "src" {
		t.Errorf("trigger event missing/wrong: %+v", rt.Requests[0].TriggerEvent)
	}
	if got := sink.ByType(EventTriggerFired); len(got) != 1 {
		t.Errorf("expected one fired event, got %d", len(got))
	}
}

func TestDispatcherFireDropConcurrencyEmitsDroppedEvent(t *testing.T) {
	sink := &MemorySink{}
	rt := &FakeRuntime{Err: ErrDropConcurrency}
	d := &Dispatcher{Runtime: rt, Sink: sink, DLQ: NoopDLQ{}}
	sub := Subscription{WorkflowID: "wf-1", TriggerID: "src", Type: ast.TriggerEventBus}
	_, err := d.Fire(context.Background(), sub, nil)
	if !errors.Is(err, ErrDropConcurrency) {
		t.Fatalf("expected drop_concurrency, got %v", err)
	}
	if got := sink.ByType(EventTriggerDropped); len(got) != 1 {
		t.Errorf("expected dropped event, got %d", len(got))
	}
}

func TestWebhookHandlerVerifiesSignatureAndDispatches(t *testing.T) {
	reg := NewRegistry()
	reg.IngestWorkflow("t1", "w1", sampleWorkflow(ast.Trigger{
		ID:     "gh",
		Type:   ast.TriggerWebhookIn,
		Config: map[string]any{"secret_ref": "ws:secret:gh-hook"},
	}))
	rt := &FakeRuntime{Resp: StartExecutionResponse{ExecutionID: "exec-1"}}
	d := &Dispatcher{Runtime: rt, Sink: &MemorySink{}, DLQ: NoopDLQ{}}
	handler := &WebhookHandler{
		Registry:   reg,
		Dispatcher: d,
		Secrets:    StaticSecrets{"ws:secret:gh-hook": "topsecret"},
	}

	body := []byte(`{"action":"opened","number":42}`)
	mac := hmac.New(sha256.New, []byte("topsecret"))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/in/wf-1/gh", strings.NewReader(string(body)))
	req.Header.Set("X-Forge-Signature", sig)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status: %d body=%s", w.Code, w.Body.String())
	}
	if len(rt.Requests) != 1 {
		t.Errorf("expected dispatch, got none")
	}
	var out map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["execution_id"] != "exec-1" {
		t.Errorf("execution_id: %v", out)
	}
}

func TestWebhookHandlerRejectsInvalidSignature(t *testing.T) {
	reg := NewRegistry()
	reg.IngestWorkflow("t1", "w1", sampleWorkflow(ast.Trigger{
		ID: "gh", Type: ast.TriggerWebhookIn,
		Config: map[string]any{"secret_ref": "ws:secret:gh-hook"},
	}))
	handler := &WebhookHandler{
		Registry: reg,
		Dispatcher: &Dispatcher{Runtime: &FakeRuntime{}, Sink: &MemorySink{}, DLQ: NoopDLQ{}},
		Secrets:    StaticSecrets{"ws:secret:gh-hook": "topsecret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/hooks/in/wf-1/gh", strings.NewReader("{}"))
	req.Header.Set("X-Forge-Signature", "sha256=deadbeef")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestEventBusRouterRefusesUnknownTopic(t *testing.T) {
	reg := NewRegistry()
	reg.IngestWorkflow("t1", "w1", sampleWorkflow(ast.Trigger{
		ID: "bad", Type: ast.TriggerEventBus,
		Config: map[string]any{"topic": "unknown.topic.v1"},
	}))
	rt := &FakeRuntime{}
	d := &Dispatcher{Runtime: rt, Sink: &MemorySink{}, DLQ: NoopDLQ{}}
	bus := NewChannelBus()
	router := NewEventBusRouter(reg, d, bus, map[string]struct{}{
		"github.push.v1": {}, // bad topic intentionally missing
	})
	router.Refresh()
	if router.SubscribedCount() != 0 {
		t.Errorf("expected 0 subs for unknown topic, got %d", router.SubscribedCount())
	}
}

func TestEventBusRouterDispatchesOnKnownTopic(t *testing.T) {
	reg := NewRegistry()
	reg.IngestWorkflow("t1", "w1", sampleWorkflow(ast.Trigger{
		ID: "src", Type: ast.TriggerEventBus,
		Config: map[string]any{"topic": "github.push.v1"},
	}))
	rt := &FakeRuntime{Resp: StartExecutionResponse{ExecutionID: "exec-9"}}
	sink := &MemorySink{}
	d := &Dispatcher{Runtime: rt, Sink: sink, DLQ: NoopDLQ{}}
	bus := NewChannelBus()
	router := NewEventBusRouter(reg, d, bus, map[string]struct{}{"github.push.v1": {}})
	router.Refresh()
	if router.SubscribedCount() != 1 {
		t.Fatalf("expected 1 sub, got %d", router.SubscribedCount())
	}
	bus.Publish("github.push.v1", map[string]any{"repo": "acme/foo"})
	if len(rt.Requests) != 1 {
		t.Errorf("expected one dispatch, got %d", len(rt.Requests))
	}
	if rt.Requests[0].TriggerEvent.Payload["repo"] != "acme/foo" {
		t.Errorf("payload not forwarded: %+v", rt.Requests[0].TriggerEvent.Payload)
	}
}

func TestEmailPollerDispatchesMatchingMessages(t *testing.T) {
	reg := NewRegistry()
	reg.IngestWorkflow("t1", "w1", sampleWorkflow(ast.Trigger{
		ID: "support", Type: ast.TriggerEmailInbound,
		Config: map[string]any{
			"mailbox_ref": "ws:mailbox:support",
			"filter":      map[string]any{"subject_contains": "[urgent]"},
		},
	}))
	rt := &FakeRuntime{Resp: StartExecutionResponse{ExecutionID: "exec-7"}}
	d := &Dispatcher{Runtime: rt, Sink: &MemorySink{}, DLQ: NoopDLQ{}}
	mailbox := &FixtureMailbox{Messages: []EmailMessage{
		{MessageID: "1", Subject: "[urgent] outage", From: "alice@acme.com", Body: "help", ReceivedAt: time.Now()},
		{MessageID: "2", Subject: "fyi: report", From: "bob@acme.com", Body: "info", ReceivedAt: time.Now()},
	}}
	poller := NewEmailPoller(reg, d, mailbox)
	n := poller.Tick(context.Background())
	if n != 1 {
		t.Errorf("expected 1 dispatch (matching filter), got %d", n)
	}
	if len(rt.Requests) != 1 {
		t.Fatalf("dispatches: %d", len(rt.Requests))
	}
	got := rt.Requests[0].TriggerEvent.Payload
	if got["subject"] != "[urgent] outage" {
		t.Errorf("subject: %v", got)
	}
}

func TestCronSchedulerSchedulesAndDispatches(t *testing.T) {
	reg := NewRegistry()
	rt := &FakeRuntime{Resp: StartExecutionResponse{ExecutionID: "exec-cron"}}
	d := &Dispatcher{Runtime: rt, Sink: &MemorySink{}, DLQ: NoopDLQ{}}
	cron := NewCronScheduler(reg, d)
	cron.Start()
	defer cron.Stop()

	reg.IngestWorkflow("t1", "w1", sampleWorkflow(ast.Trigger{
		ID: "tick", Type: ast.TriggerCron,
		Config: map[string]any{"expression": "* * * * * *"}, // every second
	}))
	cron.Refresh()
	if cron.ScheduledCount() != 1 {
		t.Fatalf("expected 1 scheduled entry, got %d", cron.ScheduledCount())
	}
	// Wait up to 2s for the cron to fire.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && len(rt.Requests) == 0 {
		time.Sleep(50 * time.Millisecond)
	}
	if len(rt.Requests) == 0 {
		t.Fatalf("expected at least one dispatch from cron within 2s")
	}
}
