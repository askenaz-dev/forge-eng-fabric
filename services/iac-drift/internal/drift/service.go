package drift

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type TerraformPlanner interface {
	Plan(ctx context.Context, ws Workspace) (PlanResult, error)
}

type Remediator interface {
	Propose(ctx context.Context, finding Finding) (prURL string, err error)
}

type Service struct {
	Store      *Store
	Sink       Sink
	Planner    TerraformPlanner
	Remediator Remediator
	Now        func() time.Time
}

func NewService(store *Store, sink Sink) *Service {
	return &Service{Store: store, Sink: sink, Planner: StaticPlanner{}, Now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) RunScheduler(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = s.RunOnce(context.Background())
		case <-stop:
			return
		}
	}
}

func (s *Service) RunOnce(ctx context.Context) error {
	var errs []error
	for _, ws := range s.Store.Workspaces() {
		if err := s.CheckWorkspace(ctx, ws); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) CheckWorkspace(ctx context.Context, ws Workspace) error {
	ignore, err := s.loadIgnore(ws)
	if err != nil {
		_ = s.Sink.Emit(newEvent(ws, "iac.drift.suppression.invalid.v1", map[string]any{"reason": err.Error()}))
		return err
	}
	res, err := s.Planner.Plan(ctx, ws)
	if err != nil {
		return err
	}
	if res.ExitCode != 2 {
		return nil
	}
	for _, ch := range res.Changes {
		f := Finding{
			TenantID: ws.TenantID, WorkspaceID: ws.WorkspaceID, RuntimeID: ws.RuntimeID,
			Resource: ch.Resource, Field: ch.Field, Expected: ch.Expected, Actual: ch.Actual,
			Severity: classify(ch), Suppressed: ignore.Suppresses(ch, s.Now()),
		}
		f = s.Store.InsertFinding(f)
		if !f.Suppressed {
			_ = s.Sink.Emit(newEvent(ws, "iac.drift.detected.v1", map[string]any{
				"finding_id": f.ID, "runtime_id": f.RuntimeID, "resource": f.Resource, "field": f.Field,
				"expected": f.Expected, "actual": f.Actual, "severity": string(f.Severity),
			}))
			if s.Remediator != nil {
				prURL, err := s.Remediator.Propose(ctx, f)
				if err != nil {
					return err
				}
				s.Store.SetRemediationPRURL(f.ID, prURL)
				_ = s.Sink.Emit(newEvent(ws, "iac.drift.remediation.proposed.v1", map[string]any{
					"finding_id": f.ID, "remediation_pr_url": prURL, "severity": string(f.Severity),
				}))
			}
		}
	}
	return nil
}

func (s *Service) loadIgnore(ws Workspace) (IgnoreFile, error) {
	path := filepath.Join(ws.RepoPath, ".forge-drift-ignore.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return IgnoreFile{}, nil
		}
		return IgnoreFile{}, err
	}
	return ParseIgnoreFile(data)
}

func classify(ch PlanChange) Severity {
	v := strings.ToLower(ch.Resource + "." + ch.Field)
	switch {
	case strings.Contains(v, "iam") || strings.Contains(v, "firewall") || strings.Contains(v, "network"):
		return SeverityHigh
	case strings.Contains(v, "node_pool") || strings.Contains(v, "cluster") || strings.Contains(v, "service"):
		return SeverityMedium
	case strings.Contains(v, "label") || strings.Contains(v, "tag"):
		return SeverityLow
	default:
		return SeverityMedium
	}
}

type StaticPlanner struct{}

func (StaticPlanner) Plan(context.Context, Workspace) (PlanResult, error) {
	return PlanResult{ExitCode: 0}, nil
}
