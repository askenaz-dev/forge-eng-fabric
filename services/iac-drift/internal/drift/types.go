package drift

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type Workspace struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	WorkspaceID  string `json:"workspace_id"`
	RuntimeID    string `json:"runtime_id"`
	RepoPath     string `json:"repo_path"`
	TerraformDir string `json:"terraform_dir"`
}

type Finding struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	WorkspaceID      string    `json:"workspace_id"`
	RuntimeID        string    `json:"runtime_id"`
	Resource         string    `json:"resource"`
	Field            string    `json:"field"`
	Expected         string    `json:"expected"`
	Actual           string    `json:"actual"`
	Severity         Severity  `json:"severity"`
	Suppressed       bool      `json:"suppressed"`
	RemediationPRURL string    `json:"remediation_pr_url,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type PlanChange struct {
	Resource string
	Field    string
	Expected string
	Actual   string
}

type PlanResult struct {
	ExitCode int
	Changes  []PlanChange
	Stdout   string
	Stderr   string
}

type Store struct {
	mu         sync.RWMutex
	workspaces map[string]Workspace
	findings   map[string]Finding
}

func NewStore() *Store {
	return &Store{workspaces: map[string]Workspace{}, findings: map[string]Finding{}}
}

func (s *Store) UpsertWorkspace(ws Workspace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ws.ID == "" {
		ws.ID = newID()
	}
	s.workspaces[ws.ID] = ws
}

func (s *Store) Workspaces() []Workspace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Workspace, 0, len(s.workspaces))
	for _, ws := range s.workspaces {
		out = append(out, ws)
	}
	return out
}

func (s *Store) InsertFinding(f Finding) Finding {
	if f.ID == "" {
		f.ID = newID()
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.findings[f.ID] = f
	return f
}

func (s *Store) SetRemediationPRURL(id, url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.findings[id]
	if !ok {
		return
	}
	f.RemediationPRURL = url
	s.findings[id] = f
}

func (s *Store) Findings() []Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Finding, 0, len(s.findings))
	for _, f := range s.findings {
		out = append(out, f)
	}
	return out
}

func newID() string { return uuid.NewString() }
