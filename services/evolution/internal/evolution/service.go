package evolution

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Deriver runs the `derive-openspec-evolution` skill against a postmortem.
// The default impl is rule-based; production wires this to an LLM call.
type Deriver interface {
	Derive(ctx context.Context, in PostmortemInput) (*Proposal, error)
}

// OpenSpecClient writes converted proposals to the OpenSpec backbone.
type OpenSpecClient interface {
	CreateChange(ctx context.Context, p *Proposal) (changeID string, err error)
}

// Service ties the deriver, store and sink together.
type Service struct {
	Store    *Store
	Sink     Sink
	Deriver  Deriver
	OpenSpec OpenSpecClient
	Now      func() time.Time
}

// NewService creates a Service with sensible defaults.
func NewService(store *Store, sink Sink) *Service {
	if sink == nil {
		sink = &MemorySink{}
	}
	return &Service{
		Store:    store,
		Sink:     sink,
		Deriver:  &RuleBasedDeriver{},
		OpenSpec: &noopOpenSpec{},
		Now:      func() time.Time { return time.Now().UTC() },
	}
}

// FromPostmortem derives a proposal and saves it in Inbox status.
func (s *Service) FromPostmortem(ctx context.Context, in PostmortemInput) (*Proposal, error) {
	if in.IncidentID == "" {
		return nil, fmt.Errorf("incident_id required")
	}
	p, err := s.Deriver.Derive(ctx, in)
	if err != nil {
		return nil, err
	}
	p.ID = "evo-" + uuid.NewString()
	p.IncidentID = in.IncidentID
	p.TenantID = in.TenantID
	p.WorkspaceID = in.WorkspaceID
	p.AssetID = in.AssetID
	p.PostmortemURL = in.PostmortemURL
	p.Source = AutonomousLoopMarker
	p.SkillVersion = SkillVersion
	p.Status = StatusInbox
	p.Synthetic = in.Synthetic
	p.CreatedAt = s.Now()
	s.Store.Save(p)
	_ = s.Sink.Emit(newEvent(p.TenantID, p.WorkspaceID, "evolution.openspec_proposal.v1",
		"proposal/"+p.ID, map[string]any{
			"proposal_id":      p.ID,
			"incident_id":      p.IncidentID,
			"asset_id":         p.AssetID,
			"source":           p.Source,
			"skill_version":    p.SkillVersion,
			"suggestion_count": len(p.Suggestions),
			"synthetic":        p.Synthetic,
		}))
	return p, nil
}

// Review applies a human review decision and (when approved) converts to an
// OpenSpec change.
func (s *Service) Review(ctx context.Context, id string, decision ReviewDecision) (*Proposal, error) {
	p := s.Store.Get(id)
	if p == nil {
		return nil, fmt.Errorf("proposal_not_found")
	}
	if p.Status != StatusInbox {
		return nil, fmt.Errorf("invalid_status: %s", p.Status)
	}
	if !decision.Approved {
		p.Status = StatusRejected
		s.Store.Save(p)
		return p, nil
	}
	p.Status = StatusAccepted
	s.Store.Save(p)
	changeID, err := s.OpenSpec.CreateChange(ctx, p)
	if err != nil {
		return nil, err
	}
	p.OpenSpecChange = changeID
	p.Status = StatusConverted
	s.Store.Save(p)
	return p, nil
}

// --- defaults ---

// RuleBasedDeriver produces a deterministic proposal that names the lessons
// learned, the citations, and the healing actions used. Good enough for tests
// and synthetic flows; the production deriver runs an LLM through LiteLLM.
type RuleBasedDeriver struct{}

func (r *RuleBasedDeriver) Derive(_ context.Context, in PostmortemInput) (*Proposal, error) {
	suggestions := []Suggestion{}
	for _, lesson := range in.Lessons {
		suggestions = append(suggestions, Suggestion{
			Kind:   KindAcceptanceCriteria,
			Title:  truncate("AC: "+lesson, 80),
			Detail: lesson,
		})
	}
	suggestions = append(suggestions, Suggestion{
		Kind:   KindRunbookUpdate,
		Title:  "Document healing actions used during this incident",
		Detail: "Healing actions: " + strings.Join(in.HealingActions, ", "),
	})
	if in.Severity == "critical" {
		suggestions = append(suggestions, Suggestion{
			Kind:   KindNewGate,
			Title:  "Add deploy gate for the failing condition",
			Detail: "Severity-critical incidents should gate future deploys until the failing condition is detected automatically.",
		})
	}
	if len(in.HealingActions) == 0 {
		suggestions = append(suggestions, Suggestion{
			Kind:   KindNewHealingAction,
			Title:  "Add a healing action for this incident type",
			Detail: "No healing action existed for this incident; propose adding one to the catalog.",
		})
	}
	title := fmt.Sprintf("OpenSpec evolution from postmortem of %s", in.IncidentID)
	why := fmt.Sprintf(
		"Lessons from incident %s on service %s/%s. Root cause: %s",
		in.IncidentID, in.Service, in.Environment, in.RootCause,
	)
	return &Proposal{
		Title:       title,
		Why:         why,
		Suggestions: suggestions,
	}, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

type noopOpenSpec struct{}

func (n *noopOpenSpec) CreateChange(_ context.Context, p *Proposal) (string, error) {
	return "change-" + p.ID, nil
}
