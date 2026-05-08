package onboarding

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tpl "github.com/forge-eng-fabric/services/scaffolder/pkg/template"
)

// PolicyDecision mirrors the policy-engine response shape (subset).
type PolicyDecision struct {
	Decision          string   `json:"decision"`
	Rationale         string   `json:"rationale"`
	RequiredApprovers []string `json:"required_approvers,omitempty"`
}

// PolicyChecker decides if an onboarding may proceed.
type PolicyChecker interface {
	CheckOnboarding(ctx context.Context, req *Request) (PolicyDecision, error)
}

// AlwaysAllowPolicy is the test/dev default.
type AlwaysAllowPolicy struct{}

func (AlwaysAllowPolicy) CheckOnboarding(_ context.Context, _ *Request) (PolicyDecision, error) {
	return PolicyDecision{Decision: "allow", Rationale: "default-allow"}, nil
}

// TemplateCatalog returns the manifest path for an approved template.
type TemplateCatalog interface {
	Resolve(ctx context.Context, id, version string) (manifestPath string, lifecycle string, trust string, err error)
}

type TemplateLister interface {
	List(ctx context.Context) ([]TemplateSummary, error)
}

// FilesystemCatalog resolves manifests from a base directory like
// `forge-templates/templates/<id>/<version>/template.yaml`. Lifecycle/trust
// are static defaults — production replaces this with the registry-backed
// catalog.
type FilesystemCatalog struct {
	BaseDir string
}

func (c FilesystemCatalog) Resolve(_ context.Context, id, version string) (string, string, string, error) {
	path := filepath.Join(c.BaseDir, id, version, "template.yaml")
	return path, "approved", "T3", nil
}

func (c FilesystemCatalog) List(_ context.Context) ([]TemplateSummary, error) {
	templateDirs, err := os.ReadDir(c.BaseDir)
	if err != nil {
		return nil, err
	}
	out := []TemplateSummary{}
	for _, templateDir := range templateDirs {
		if !templateDir.IsDir() {
			continue
		}
		versionDirs, err := os.ReadDir(filepath.Join(c.BaseDir, templateDir.Name()))
		if err != nil {
			return nil, err
		}
		for _, versionDir := range versionDirs {
			if !versionDir.IsDir() {
				continue
			}
			manifestPath := filepath.Join(c.BaseDir, templateDir.Name(), versionDir.Name(), "template.yaml")
			manifest, err := tpl.Load(manifestPath)
			if err != nil {
				return nil, err
			}
			params := map[string]TemplateParameter{}
			for name, spec := range manifest.Parameters {
				params[name] = TemplateParameter{
					Type:        spec.Type,
					Description: spec.Description,
					Required:    spec.Required,
					Default:     spec.Default,
					Pattern:     spec.Pattern,
					Enum:        spec.Enum,
				}
			}
			out = append(out, TemplateSummary{
				ID:                   manifest.ID,
				Version:              manifest.Version,
				Description:          manifest.Description,
				Category:             manifest.Category,
				LifecycleState:       "approved",
				TrustLevel:           "T3",
				Parameters:           params,
				RequiredCapabilities: append([]string(nil), manifest.RequiredCapabilities...),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].ID < out[j].ID
		}
		return out[i].Category < out[j].Category
	})
	return out, nil
}

// GitHubMCPClient invokes the GitHub MCP write-mode tools. Concrete
// implementation lives in `mcpclient.go`.
type GitHubMCPClient interface {
	CreateRepo(ctx context.Context, req *Request) (repoURL string, err error)
	SetCodeowners(ctx context.Context, req *Request, content string) error
	AddPRTemplate(ctx context.Context, req *Request, content string) error
	SetBranchProtection(ctx context.Context, req *Request, rules map[string]any) error
	SetRequiredChecks(ctx context.Context, req *Request, checks []string) error
}

// AssetRegistrar records the new application asset in the Registry.
type AssetRegistrar interface {
	RegisterApplication(ctx context.Context, req *Request, repoURL string, metadata map[string]any) (assetID string, err error)
}

// NoopRegistrar is a default that returns a deterministic asset id.
type NoopRegistrar struct{}

func (NoopRegistrar) RegisterApplication(_ context.Context, req *Request, _ string, _ map[string]any) (string, error) {
	return "app/" + req.WorkspaceID + "/" + req.RepoName, nil
}

// Service wires the dependencies and exposes Submit + Replay.
type Service struct {
	Store      *Store
	Sink       Sink
	Policy     PolicyChecker
	Catalog    TemplateCatalog
	GitHub     GitHubMCPClient
	Registrar  AssetRegistrar
	WorkOutDir string // base dir for scaffolder output (per-request subdir)
}

func NewService(store *Store, sink Sink) *Service {
	if sink == nil {
		sink = &MemorySink{}
	}
	return &Service{
		Store:      store,
		Sink:       sink,
		Policy:     AlwaysAllowPolicy{},
		Catalog:    FilesystemCatalog{BaseDir: "forge-templates/templates"},
		GitHub:     &StubGitHubMCP{},
		Registrar:  NoopRegistrar{},
		WorkOutDir: filepath.Join(".", "tmp", "onboarding"),
	}
}

// Submit creates a new request and (if not pending approval) runs the full
// flow synchronously. Production may run async — the interface contract is
// the same: callers poll `GET /v1/onboarding/{id}` for live status.
func (s *Service) Submit(ctx context.Context, req *Request) (*Request, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	stored, created, err := s.Store.Insert(req)
	if err != nil {
		return nil, err
	}
	if !created {
		return stored, nil // idempotent return
	}

	_ = s.Sink.Emit(newCloudEvent(stored, "com.forge.app.onboarding_requested.v1",
		fmt.Sprintf("workspace/%s/repo/%s", stored.WorkspaceID, stored.RepoName),
		map[string]any{
			"workspace_id": stored.WorkspaceID,
			"repo_name":    stored.RepoName,
			"template":     stored.TemplateID + "@" + stored.TemplateVersion,
			"criticality":  stored.Criticality,
			"requested_by": stored.RequestedBy,
		}))
	s.Store.AppendEvent(stored.ID, "request.received", OutcomeStarted, "request received", nil, 0)

	go s.run(context.Background(), stored)
	return stored, nil
}

func (s *Service) ListTemplates(ctx context.Context) ([]TemplateSummary, error) {
	lister, ok := s.Catalog.(TemplateLister)
	if !ok {
		return nil, fmt.Errorf("template catalog does not support listing")
	}
	return lister.List(ctx)
}

func (s *Service) run(ctx context.Context, req *Request) {
	stages := []struct {
		name string
		fn   func(context.Context, *Request) (map[string]any, error)
	}{
		{"policy.evaluate", s.stagePolicy},
		{"template.resolve", s.stageResolveTemplate},
		{"scaffold.render", s.stageScaffold},
		{"github.create_repo", s.stageCreateRepo},
		{"github.codeowners", s.stageCodeowners},
		{"github.pr_template", s.stagePRTemplate},
		{"github.branch_protection", s.stageBranchProtection},
		{"github.required_checks", s.stageRequiredChecks},
		{"asset.register", s.stageRegisterAsset},
	}

	s.Store.UpdateStatus(req.ID, StatusRunning, "")
	for _, st := range stages {
		start := time.Now()
		s.Store.AppendEvent(req.ID, st.name, OutcomeStarted, "", nil, 0)
		payload, err := st.fn(ctx, req)
		duration := time.Since(start)
		if err != nil {
			s.Store.AppendEvent(req.ID, st.name, OutcomeFailed, err.Error(), payload, duration)
			s.Store.UpdateStatus(req.ID, StatusFailed, fmt.Sprintf("%s: %v", st.name, err))
			_ = s.Sink.Emit(newCloudEvent(req, "com.forge.app.onboarding_failed.v1",
				fmt.Sprintf("request/%s", req.ID),
				map[string]any{"stage": st.name, "error": err.Error()}))
			return
		}
		s.Store.AppendEvent(req.ID, st.name, OutcomeCompleted, "", payload, duration)
	}

	s.Store.UpdateStatus(req.ID, StatusCompleted, "")
	_ = s.Sink.Emit(newCloudEvent(req, "com.forge.app.onboarding_completed.v1",
		fmt.Sprintf("request/%s", req.ID),
		map[string]any{
			"workspace_id": req.WorkspaceID,
			"repo_name":    req.RepoName,
			"asset_id":     req.AssetID,
		}))
}

func (s *Service) stagePolicy(ctx context.Context, req *Request) (map[string]any, error) {
	dec, err := s.Policy.CheckOnboarding(ctx, req)
	if err != nil {
		return nil, err
	}
	if dec.Decision == "deny" {
		return map[string]any{"rationale": dec.Rationale}, fmt.Errorf("policy denied: %s", dec.Rationale)
	}
	if dec.Decision == "requires_approval" {
		s.Store.UpdateStatus(req.ID, StatusPendingApproval, dec.Rationale)
		return map[string]any{"required_approvers": dec.RequiredApprovers}, fmt.Errorf("requires_approval: %s", dec.Rationale)
	}
	return map[string]any{"decision": dec.Decision}, nil
}

func (s *Service) stageResolveTemplate(ctx context.Context, req *Request) (map[string]any, error) {
	manifestPath, lifecycle, trust, err := s.Catalog.Resolve(ctx, req.TemplateID, req.TemplateVersion)
	if err != nil {
		return nil, err
	}
	if lifecycle != "approved" {
		return nil, fmt.Errorf("template_not_approved: lifecycle=%s", lifecycle)
	}
	if !strings.HasPrefix(trust, "T") || trust < "T3" {
		return nil, fmt.Errorf("template_trust_too_low: trust=%s", trust)
	}
	return map[string]any{"manifest": manifestPath, "lifecycle": lifecycle, "trust": trust}, nil
}

func (s *Service) stageScaffold(ctx context.Context, req *Request) (map[string]any, error) {
	manifestPath, _, _, _ := s.Catalog.Resolve(ctx, req.TemplateID, req.TemplateVersion)
	m, err := tpl.Load(manifestPath)
	if err != nil {
		return nil, err
	}
	outDir := filepath.Join(s.WorkOutDir, req.ID)
	res, err := m.Render(req.Parameters, map[string]any{
		"owner":               firstOwner(req.Owners),
		"criticality":         req.Criticality,
		"data_classification": req.DataClassification,
	}, outDir)
	if err != nil {
		return nil, err
	}
	return map[string]any{"out": res.OutputDir, "files_written": res.FilesWritten}, nil
}

func (s *Service) stageCreateRepo(ctx context.Context, req *Request) (map[string]any, error) {
	url, err := s.GitHub.CreateRepo(ctx, req)
	if err != nil {
		return nil, err
	}
	_ = s.Sink.Emit(newCloudEvent(req, "com.forge.repo.created.v1",
		fmt.Sprintf("repo/%s/%s", req.RepoOrg, req.RepoName),
		map[string]any{"repo_url": url, "default_branch": "main"}))
	return map[string]any{"repo_url": url}, nil
}

func (s *Service) stageCodeowners(ctx context.Context, req *Request) (map[string]any, error) {
	content := buildCodeowners(req.Owners)
	if err := s.GitHub.SetCodeowners(ctx, req, content); err != nil {
		return nil, err
	}
	return map[string]any{"codeowners_lines": strings.Count(content, "\n")}, nil
}

func (s *Service) stagePRTemplate(ctx context.Context, req *Request) (map[string]any, error) {
	tpl := defaultPRTemplate(req)
	if err := s.GitHub.AddPRTemplate(ctx, req, tpl); err != nil {
		return nil, err
	}
	return map[string]any{"pr_template_bytes": len(tpl)}, nil
}

func (s *Service) stageBranchProtection(ctx context.Context, req *Request) (map[string]any, error) {
	rules := branchProtectionRules(req.Criticality)
	if err := s.GitHub.SetBranchProtection(ctx, req, rules); err != nil {
		return nil, err
	}
	_ = s.Sink.Emit(newCloudEvent(req, "com.forge.branch_protection_applied.v1",
		fmt.Sprintf("repo/%s/%s", req.RepoOrg, req.RepoName),
		map[string]any{"branch": "main", "rules": rules}))
	return map[string]any{"rules": rules}, nil
}

func (s *Service) stageRequiredChecks(ctx context.Context, req *Request) (map[string]any, error) {
	checks := baselineChecks()
	if err := s.GitHub.SetRequiredChecks(ctx, req, checks); err != nil {
		return nil, err
	}
	return map[string]any{"required_checks": checks}, nil
}

func (s *Service) stageRegisterAsset(ctx context.Context, req *Request) (map[string]any, error) {
	repoURL := fmt.Sprintf("https://github.com/%s/%s", req.RepoOrg, req.RepoName)
	imageRepo := imageRepository(req)
	assetID, err := s.Registrar.RegisterApplication(ctx, req, repoURL, map[string]any{
		"template_id":         req.TemplateID,
		"template_version":    req.TemplateVersion,
		"runtime":             req.Parameters["runtime"],
		"criticality":         req.Criticality,
		"data_classification": req.DataClassification,
		"owners":              req.Owners,
		"repo_url":            repoURL,
		"default_branch":      "main",
		"image_repository":    imageRepo,
		"image": map[string]any{
			"repository":           imageRepo,
			"signature_verified":   false,
			"attestation_verified": false,
		},
	})
	if err != nil {
		return nil, err
	}
	s.Store.SetAsset(req.ID, assetID)
	return map[string]any{"asset_id": assetID, "lifecycle_state": "proposed"}, nil
}

func imageRepository(req *Request) string {
	if configured, ok := req.Parameters["image_repository"].(string); ok && configured != "" {
		return configured
	}
	return fmt.Sprintf("artifact-registry.local/%s/%s", req.WorkspaceID, req.RepoName)
}

func validateRequest(req *Request) error {
	if req.WorkspaceID == "" {
		return fmt.Errorf("workspace_id required")
	}
	if req.RepoOrg == "" || req.RepoName == "" {
		return fmt.Errorf("repo_org and repo_name required")
	}
	if req.TemplateID == "" || req.TemplateVersion == "" {
		return fmt.Errorf("template_id and template_version required")
	}
	if req.Criticality == "" {
		req.Criticality = "medium"
	}
	if req.DataClassification == "" {
		req.DataClassification = "internal"
	}
	if req.Parameters == nil {
		req.Parameters = map[string]any{}
	}
	if _, ok := req.Parameters["name"]; !ok {
		req.Parameters["name"] = req.RepoName
	}
	return nil
}

func firstOwner(owners []string) string {
	if len(owners) > 0 {
		return owners[0]
	}
	return "@unknown"
}

func buildCodeowners(owners []string) string {
	if len(owners) == 0 {
		return "* @unknown\n"
	}
	return "* " + strings.Join(owners, " ") + "\n"
}

func defaultPRTemplate(req *Request) string {
	return strings.Join([]string{
		"## Summary",
		"",
		"## OpenSpec",
		"OpenSpec: <id>",
		"",
		"## Risk",
		fmt.Sprintf("Criticality: %s", req.Criticality),
		"",
	}, "\n")
}

func branchProtectionRules(criticality string) map[string]any {
	required := 1
	signed := false
	if criticality == "high" || criticality == "critical" {
		required = 2
		signed = true
	}
	return map[string]any{
		"require_pr_review":    true,
		"min_reviewers":        required,
		"dismiss_stale":        true,
		"require_code_owners":  true,
		"linear_history":       true,
		"signed_commits":       signed,
		"restrict_push_to_app": true,
	}
}

func baselineChecks() []string {
	return []string{
		"forge/lint",
		"forge/test-with-coverage",
		"forge/sast",
		"forge/sca",
		"forge/sbom",
		"forge/container-scan",
		"forge/cosign-sign-attest",
		"forge/openspec-link",
	}
}
