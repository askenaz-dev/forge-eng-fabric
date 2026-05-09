package registry

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
	"github.com/forge-eng-fabric/pkg/workflow/dsl"
)

func parseSeedYAML(raw []byte) (*ast.Workflow, error) {
	return dsl.Parse(raw)
}

// SeedDirectory loads every YAML file from `dir`, registers a parent workflow
// per file (idempotent — existing workflows are not duplicated), and publishes
// each as a `published` version. Failure to seed a single file is logged and
// skipped so a malformed seed doesn't take down service startup.
//
// Used at platform startup (cmd/main.go) for reference workflows like
// `forge.reference.intent-to-deploy@1`.
func (s *Service) SeedDirectory(ctx context.Context, dir string, defaultTenant, defaultWorkspace, actor string) error {
	if dir == "" {
		return nil
	}
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // seeding is optional
	}
	if err != nil {
		return fmt.Errorf("stat seed dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("seed path is not a directory: %s", dir)
	}

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".yaml") && !strings.HasSuffix(strings.ToLower(d.Name()), ".yml") {
			return nil
		}
		if err := s.seedOne(ctx, path, defaultTenant, defaultWorkspace, actor); err != nil {
			log.Printf("workflow-registry: skip seed %s: %v", path, err)
		}
		return nil
	})
}

func (s *Service) seedOne(ctx context.Context, path, tenant, workspace, actor string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	// Use Parse to extract metadata.id without re-validating elsewhere.
	wf, err := parseSeedYAML(raw)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if wf.Metadata.ID == "" {
		return errors.New("metadata.id is empty")
	}

	tags := append([]string{}, wf.Metadata.Tags...)
	hasReference := false
	for _, t := range tags {
		if t == "reference" {
			hasReference = true
			break
		}
	}
	if !hasReference {
		tags = append(tags, "reference")
	}

	parentReq := CreateWorkflowRequest{
		ID:          wf.Metadata.ID,
		TenantID:    tenant,
		WorkspaceID: workspace,
		Name:        wf.Metadata.Name,
		Owners:      wf.Metadata.Owners,
		Description: wf.Metadata.Description,
		Tags:        tags,
		Visibility:  wf.Metadata.Visibility,
	}
	if _, err := s.CreateWorkflow(ctx, parentReq); err != nil {
		// Idempotent: if the parent already exists, fall through to publish.
		if !strings.HasPrefix(err.Error(), "workflow_already_exists") {
			return fmt.Errorf("create_workflow: %w", err)
		}
	}

	pubReq := PublishVersionRequest{
		TenantID:     tenant,
		WorkflowID:   wf.Metadata.ID,
		WorkflowYAML: string(raw),
		Actor:        actor,
		AutoBump:     false,
	}
	if _, err := s.PublishVersion(ctx, pubReq); err != nil {
		// Skip if the version already exists — the seed file's version is the
		// canonical published version for this commit.
		if errors.Is(err, ErrVersionAlreadyExists) {
			return nil
		}
		return fmt.Errorf("publish_version: %w", err)
	}
	log.Printf("workflow-registry: seeded %s@%s", wf.Metadata.ID, wf.Metadata.Version)
	return nil
}
