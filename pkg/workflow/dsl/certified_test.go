package dsl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/forge-eng-fabric/pkg/workflow/lint"
	"github.com/forge-eng-fabric/pkg/workflow/schema"
)

// TestCertifiedWorkflowsLintClean ensures every shipped certified workflow
// parses, validates against the schema and lints clean.
func TestCertifiedWorkflowsLintClean(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "forge-workflows"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(root, "*", "workflow.yaml"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Skipf("no certified workflows at %s — skipping", root)
	}
	for _, path := range matches {
		path := path
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			wf, err := Parse(raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if err := schema.Validate(wf); err != nil {
				t.Fatalf("schema: %v", err)
			}
			result := lint.Lint(wf)
			if errs := result.Errors(); len(errs) > 0 {
				t.Fatalf("lint errors: %+v", errs)
			}
		})
	}
}
