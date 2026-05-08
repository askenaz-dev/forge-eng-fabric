package traceability

import (
	"time"

	"github.com/google/uuid"
)

type NodeType string

const (
	NodeOpenSpec       NodeType = "openspec"
	NodeJiraIssue      NodeType = "jira_issue"
	NodeJiraEpic       NodeType = "jira_epic"
	NodeConfluencePage NodeType = "confluence_page"
	NodeADR            NodeType = "adr"
	NodePR             NodeType = "pr"
	NodeCommit         NodeType = "commit"
	NodeDeployment     NodeType = "deployment"
	NodeAsset          NodeType = "asset"
	NodeTestRun        NodeType = "test_run"
	NodeSLO            NodeType = "slo"
	NodeIncident       NodeType = "incident"
	NodeCostRecord     NodeType = "cost_record"
)

type Relation string

const (
	RelationDerivesFrom      Relation = "derives_from"
	RelationImplements       Relation = "implements"
	RelationValidates        Relation = "validates"
	RelationDeploys          Relation = "deploys"
	RelationMonitoredBy      Relation = "monitored_by"
	RelationCostAttributedTo Relation = "cost_attributed_to"
)

type Node struct {
	ID          string         `json:"id"`
	Type        NodeType       `json:"type"`
	ExternalID  string         `json:"external_id"`
	WorkspaceID string         `json:"workspace_id"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Link struct {
	ID          string         `json:"id"`
	FromNodeID  string         `json:"from_node"`
	ToNodeID    string         `json:"to_node"`
	Relation    Relation       `json:"relation"`
	Source      string         `json:"source"`
	SourceEvent string         `json:"source_event"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type BusEvent struct {
	Type          string         `json:"type"`
	Subject       string         `json:"subject,omitempty"`
	WorkspaceID   string         `json:"workspace_id,omitempty"`
	TenantID      string         `json:"tenant_id,omitempty"`
	Actor         string         `json:"actor,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Data          map[string]any `json:"data"`
}

type IngestionResult struct {
	NodesCreated int      `json:"nodes_created"`
	LinksCreated int      `json:"links_created"`
	OpenSpecIDs  []string `json:"openspec_ids,omitempty"`
}

type GraphResponse struct {
	OpenSpecID     string    `json:"openspec_id"`
	Depth          int       `json:"depth"`
	Nodes          []Node    `json:"nodes"`
	Links          []Link    `json:"links"`
	MaterializedAt time.Time `json:"materialized_at"`
}

type BackfillRequest struct {
	Events []BusEvent `json:"events"`
}

type BackfillResponse struct {
	EventsProcessed int      `json:"events_processed"`
	NodesCreated    int      `json:"nodes_created"`
	LinksCreated    int      `json:"links_created"`
	OpenSpecIDs     []string `json:"openspec_ids"`
}

func newID() string { return uuid.NewString() }
