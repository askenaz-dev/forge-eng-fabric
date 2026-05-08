package traceability

import (
	"context"
	"strings"
	"testing"
)

func TestPRLinkCreatesGraphAndMaterializedQuery(t *testing.T) {
	sink := &MemorySink{}
	svc := NewService(NewStore(), sink)
	result, err := svc.HandleEvent(context.Background(), BusEvent{
		Type:        "pr.linked-to-openspec.v1",
		WorkspaceID: "ws-1",
		Data: map[string]any{
			"repo":        "org/app",
			"pr_number":   "42",
			"openspec_id": "spec-7",
		},
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if result.NodesCreated != 2 || result.LinksCreated != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	graph := svc.TraceabilityForOpenSpec("spec-7", 4)
	if len(graph.Nodes) != 2 || len(graph.Links) != 1 {
		t.Fatalf("unexpected graph: %+v", graph)
	}
	if graph.Links[0].Relation != RelationImplements {
		t.Fatalf("relation = %s", graph.Links[0].Relation)
	}
	if got := len(sink.ByType(EventLinkCreated)); got != 1 {
		t.Fatalf("link events = %d", got)
	}
}

func TestDeploymentCreatesAssetAndOpenSpecLinks(t *testing.T) {
	svc := NewService(NewStore(), &MemorySink{})
	result, err := svc.HandleEvent(context.Background(), BusEvent{
		Type:        "deployment.applied.v1",
		WorkspaceID: "ws-1",
		Data: map[string]any{
			"deployment_id": "dep-9",
			"asset_id":      "app-foo",
			"openspec_ids":  []any{"spec-7"},
		},
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if result.NodesCreated != 3 || result.LinksCreated != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	graph := svc.TraceabilityForOpenSpec("spec-7", 4)
	if len(graph.Nodes) != 3 || len(graph.Links) != 2 {
		t.Fatalf("unexpected graph: %+v", graph)
	}
}

func TestBackfillProcessesHistoricalEvents(t *testing.T) {
	sink := &MemorySink{}
	svc := NewService(NewStore(), sink)
	resp, err := svc.BackfillAuditLog(context.Background(), BackfillRequest{Events: []BusEvent{
		{Type: "app.onboarding.completed.v1", WorkspaceID: "ws-1", Data: map[string]any{"asset_id": "app-foo", "openspec_id": "spec-7"}},
		{Type: "test.run.completed.v1", WorkspaceID: "ws-1", Data: map[string]any{"test_run_id": "tr-1", "openspec_id": "spec-7"}},
	}})
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if resp.EventsProcessed != 2 || resp.LinksCreated != 2 {
		t.Fatalf("unexpected backfill response: %+v", resp)
	}
	if got := len(sink.ByType(EventBackfillCompleted)); got != 1 {
		t.Fatalf("backfill events = %d", got)
	}
}

func TestMetricsExposeCoverageAndLatency(t *testing.T) {
	svc := NewService(NewStore(), &MemorySink{})
	_, _ = svc.HandleEvent(context.Background(), BusEvent{Type: "pr.linked-to-openspec.v1", WorkspaceID: "ws-1", Data: map[string]any{"pr": "org/app#1", "openspec_id": "spec-7"}})
	metrics := svc.Metrics()
	for _, name := range []string{"traceability_coverage", "traceability_query_latency_p95"} {
		if !strings.Contains(metrics, name) {
			t.Fatalf("metrics missing %s: %s", name, metrics)
		}
	}
}
