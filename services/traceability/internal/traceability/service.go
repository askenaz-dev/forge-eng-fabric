package traceability

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	Store *Store
	Sink  Sink
}

func NewService(store *Store, sink Sink) *Service {
	if sink == nil {
		sink = &MemorySink{}
	}
	return &Service{Store: store, Sink: sink}
}

func (s *Service) HandleEvent(_ context.Context, event BusEvent) (IngestionResult, error) {
	switch event.Type {
	case "pr.linked-to-openspec.v1":
		return s.ingestPRLink(event), nil
	case "deployment.applied.v1":
		return s.ingestDeployment(event), nil
	case "app.onboarding.completed.v1":
		return s.ingestAppOnboarding(event), nil
	case "jira.issue.created.v1", "jira.issue.updated.v1", "jira.issue.transitioned.v1":
		return s.ingestJiraIssue(event), nil
	case "confluence.page.created.v1", "confluence.page.updated.v1":
		return s.ingestConfluencePage(event), nil
	case "test.run.completed.v1", "test.run.failed.v1":
		return s.ingestTestRun(event), nil
	case "slo.defined.v1", "slo.updated.v1":
		return s.ingestSLO(event), nil
	case "incident.opened.v1", "incident.resolved.v1":
		return s.ingestIncident(event), nil
	case "finops.budget.threshold_reached.v1", "cost.record.created.v1":
		return s.ingestCostRecord(event), nil
	default:
		return IngestionResult{}, fmt.Errorf("unsupported_event_type: %s", event.Type)
	}
}

func (s *Service) TraceabilityForOpenSpec(openSpecID string, depth int) GraphResponse {
	if depth <= 0 {
		depth = 4
	}
	if cached, ok := s.Store.Materialized(openSpecID, depth); ok && time.Since(cached.MaterializedAt) <= 5*time.Minute {
		return cached
	}
	return s.Store.RefreshMaterialized(openSpecID, depth)
}

func (s *Service) RunMaterializedViewRefresher(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.Store.RefreshAllMaterialized(4)
		case <-stop:
			return
		}
	}
}

func (s *Service) BackfillAuditLog(ctx context.Context, request BackfillRequest) (BackfillResponse, error) {
	response := BackfillResponse{OpenSpecIDs: []string{}}
	seenSpecs := map[string]bool{}
	for _, event := range request.Events {
		result, err := s.HandleEvent(ctx, event)
		if err != nil {
			return response, err
		}
		response.EventsProcessed++
		response.NodesCreated += result.NodesCreated
		response.LinksCreated += result.LinksCreated
		for _, id := range result.OpenSpecIDs {
			seenSpecs[id] = true
		}
	}
	for id := range seenSpecs {
		response.OpenSpecIDs = append(response.OpenSpecIDs, id)
		s.Store.RefreshMaterialized(id, 4)
	}
	_ = s.Sink.Emit(newEvent(BusEvent{Type: EventBackfillCompleted}, EventBackfillCompleted, "traceability/backfill", map[string]any{
		"events_processed": response.EventsProcessed,
		"nodes_created":    response.NodesCreated,
		"links_created":    response.LinksCreated,
	}))
	return response, nil
}

func (s *Service) ingestPRLink(event BusEvent) IngestionResult {
	workspaceID := workspaceID(event)
	openSpecIDs := openspecIDs(event.Data)
	prExternalID := firstNonEmpty(stringFrom(event.Data["pr_url"]), stringFrom(event.Data["pr"]), fmt.Sprintf("%s#%s", stringFrom(event.Data["repo"]), stringFrom(event.Data["pr_number"])))
	pr, prCreated := s.upsertNode(workspaceID, NodePR, prExternalID, event.Data)
	return s.linkToOpenSpecs(event, workspaceID, pr, RelationImplements, openSpecIDs, boolCount(prCreated))
}

func (s *Service) ingestDeployment(event BusEvent) IngestionResult {
	workspaceID := workspaceID(event)
	openSpecIDs := openspecIDs(event.Data)
	deploymentID := firstNonEmpty(stringFrom(event.Data["deployment_id"]), event.Subject)
	deployment, deploymentCreated := s.upsertNode(workspaceID, NodeDeployment, deploymentID, event.Data)
	assetID := stringFrom(event.Data["asset_id"])
	createdNodes := boolCount(deploymentCreated)
	createdLinks := 0
	if assetID != "" {
		asset, assetCreated := s.upsertNode(workspaceID, NodeAsset, assetID, map[string]any{"asset_id": assetID})
		createdNodes += boolCount(assetCreated)
		if s.upsertLink(event, deployment, asset, RelationDeploys) {
			createdLinks++
		}
	}
	linked := s.linkToOpenSpecs(event, workspaceID, deployment, RelationDerivesFrom, openSpecIDs, createdNodes)
	linked.LinksCreated += createdLinks
	return linked
}

func (s *Service) ingestAppOnboarding(event BusEvent) IngestionResult {
	workspaceID := workspaceID(event)
	openSpecIDs := openspecIDs(event.Data)
	assetID := firstNonEmpty(stringFrom(event.Data["asset_id"]), stringFrom(event.Data["app_id"]), event.Subject)
	asset, created := s.upsertNode(workspaceID, NodeAsset, assetID, event.Data)
	return s.linkToOpenSpecs(event, workspaceID, asset, RelationDerivesFrom, openSpecIDs, boolCount(created))
}

func (s *Service) ingestJiraIssue(event BusEvent) IngestionResult {
	workspaceID := workspaceID(event)
	openSpecIDs := openspecIDs(event.Data)
	key := firstNonEmpty(stringFrom(event.Data["key"]), stringFrom(event.Data["issue_key"]), event.Subject)
	nodeType := NodeJiraIssue
	if strings.EqualFold(stringFrom(event.Data["issue_type"]), "epic") || strings.Contains(strings.ToLower(event.Type), "epic") {
		nodeType = NodeJiraEpic
	}
	issue, created := s.upsertNode(workspaceID, nodeType, key, event.Data)
	return s.linkToOpenSpecs(event, workspaceID, issue, RelationDerivesFrom, openSpecIDs, boolCount(created))
}

func (s *Service) ingestConfluencePage(event BusEvent) IngestionResult {
	workspaceID := workspaceID(event)
	openSpecIDs := openspecIDs(event.Data)
	pageID := firstNonEmpty(stringFrom(event.Data["page_id"]), stringFrom(event.Data["url"]), event.Subject)
	page, created := s.upsertNode(workspaceID, NodeConfluencePage, pageID, event.Data)
	return s.linkToOpenSpecs(event, workspaceID, page, RelationDerivesFrom, openSpecIDs, boolCount(created))
}

func (s *Service) ingestTestRun(event BusEvent) IngestionResult {
	workspaceID := workspaceID(event)
	openSpecIDs := openspecIDs(event.Data)
	testRunID := firstNonEmpty(stringFrom(event.Data["test_run_id"]), event.Subject)
	testRun, created := s.upsertNode(workspaceID, NodeTestRun, testRunID, event.Data)
	return s.linkToOpenSpecs(event, workspaceID, testRun, RelationValidates, openSpecIDs, boolCount(created))
}

func (s *Service) ingestSLO(event BusEvent) IngestionResult {
	workspaceID := workspaceID(event)
	openSpecIDs := openspecIDs(event.Data)
	sloID := firstNonEmpty(stringFrom(event.Data["slo_id"]), event.Subject)
	slo, created := s.upsertNode(workspaceID, NodeSLO, sloID, event.Data)
	return s.linkFromOpenSpecs(event, workspaceID, slo, RelationMonitoredBy, openSpecIDs, boolCount(created))
}

func (s *Service) ingestIncident(event BusEvent) IngestionResult {
	workspaceID := workspaceID(event)
	openSpecIDs := openspecIDs(event.Data)
	incidentID := firstNonEmpty(stringFrom(event.Data["incident_id"]), event.Subject)
	incident, created := s.upsertNode(workspaceID, NodeIncident, incidentID, event.Data)
	return s.linkToOpenSpecs(event, workspaceID, incident, RelationDerivesFrom, openSpecIDs, boolCount(created))
}

func (s *Service) ingestCostRecord(event BusEvent) IngestionResult {
	workspaceID := workspaceID(event)
	openSpecIDs := openspecIDs(event.Data)
	recordID := firstNonEmpty(stringFrom(event.Data["cost_record_id"]), stringFrom(event.Data["budget_id"]), event.Subject)
	record, created := s.upsertNode(workspaceID, NodeCostRecord, recordID, event.Data)
	return s.linkToOpenSpecs(event, workspaceID, record, RelationCostAttributedTo, openSpecIDs, boolCount(created))
}

func (s *Service) linkToOpenSpecs(event BusEvent, workspaceID string, from Node, relation Relation, openSpecIDs []string, nodesCreated int) IngestionResult {
	result := IngestionResult{NodesCreated: nodesCreated, OpenSpecIDs: openSpecIDs}
	for _, openSpecID := range openSpecIDs {
		openSpec, created := s.upsertNode(workspaceID, NodeOpenSpec, openSpecID, map[string]any{"openspec_id": openSpecID})
		result.NodesCreated += boolCount(created)
		if s.upsertLink(event, from, openSpec, relation) {
			result.LinksCreated++
		}
		s.Store.RefreshMaterialized(openSpecID, 4)
	}
	return result
}

func (s *Service) linkFromOpenSpecs(event BusEvent, workspaceID string, to Node, relation Relation, openSpecIDs []string, nodesCreated int) IngestionResult {
	result := IngestionResult{NodesCreated: nodesCreated, OpenSpecIDs: openSpecIDs}
	for _, openSpecID := range openSpecIDs {
		openSpec, created := s.upsertNode(workspaceID, NodeOpenSpec, openSpecID, map[string]any{"openspec_id": openSpecID})
		result.NodesCreated += boolCount(created)
		if s.upsertLink(event, openSpec, to, relation) {
			result.LinksCreated++
		}
		s.Store.RefreshMaterialized(openSpecID, 4)
	}
	return result
}

func (s *Service) upsertNode(workspaceID string, nodeType NodeType, externalID string, metadata map[string]any) (Node, bool) {
	if externalID == "" {
		externalID = newID()
	}
	return s.Store.UpsertNode(Node{Type: nodeType, ExternalID: externalID, WorkspaceID: workspaceID, Metadata: metadata})
}

func (s *Service) upsertLink(event BusEvent, from Node, to Node, relation Relation) bool {
	link, created := s.Store.UpsertLink(Link{
		FromNodeID:  from.ID,
		ToNodeID:    to.ID,
		Relation:    relation,
		Source:      "event-bus",
		SourceEvent: event.Type,
		Metadata:    map[string]any{"correlation_id": event.CorrelationID},
	})
	if created {
		_ = s.Sink.Emit(newEvent(event, EventLinkCreated, "traceability-link/"+link.ID, map[string]any{
			"link_id":      link.ID,
			"from_node":    link.FromNodeID,
			"to_node":      link.ToNodeID,
			"relation":     link.Relation,
			"source_event": link.SourceEvent,
		}))
	}
	return created
}

func workspaceID(event BusEvent) string {
	return firstNonEmpty(event.WorkspaceID, stringFrom(event.Data["workspace_id"]))
}

func openspecIDs(data map[string]any) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(value string) {
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	add(stringFrom(data["openspec_id"]))
	add(stringFrom(data["openspec_root"]))
	switch ids := data["openspec_ids"].(type) {
	case []string:
		for _, id := range ids {
			add(id)
		}
	case []any:
		for _, id := range ids {
			add(stringFrom(id))
		}
	}
	return out
}

func stringFrom(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
