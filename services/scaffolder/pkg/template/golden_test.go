package template

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// repoRoot returns the absolute path to the workspace root by walking up from
// the test working directory until `forge-templates/` is found.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "forge-templates")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate repo root from cwd")
	return ""
}

// TestAllShippedTemplatesParseAndRender ensures each manifest in
// `forge-templates/templates/<id>/<version>/template.yaml` parses, validates,
// and renders with a representative parameter set.
func TestAllShippedTemplatesParseAndRender(t *testing.T) {
	root := repoRoot(t)
	tplBase := filepath.Join(root, "forge-templates", "templates")
	entries, err := os.ReadDir(tplBase)
	if err != nil {
		t.Fatalf("read tplBase: %v", err)
	}

	type tplCase struct {
		id       string
		manifest string
		params   map[string]any
	}
	defaultParams := func(id string) map[string]any {
		p := map[string]any{
			"name":  "svc-test",
			"owner": "@team-a",
		}
		if id == "go-library" {
			p["module_path"] = "github.com/org/svc-test"
		}
		return p
	}

	var cases []tplCase
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		versionsDir := filepath.Join(tplBase, e.Name())
		versions, _ := os.ReadDir(versionsDir)
		sort.Slice(versions, func(i, j int) bool { return versions[i].Name() > versions[j].Name() })
		for _, v := range versions {
			if !v.IsDir() {
				continue
			}
			cases = append(cases, tplCase{
				id:       e.Name() + "@" + v.Name(),
				manifest: filepath.Join(versionsDir, v.Name(), "template.yaml"),
				params:   defaultParams(e.Name()),
			})
			break
		}
	}

	if len(cases) < 5 {
		t.Fatalf("expected at least 5 shipped templates, got %d", len(cases))
	}

	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			m, err := Load(c.manifest)
			if err != nil {
				t.Fatalf("load %s: %v", c.id, err)
			}
			out := t.TempDir()
			res, err := m.Render(
				c.params,
				map[string]any{"criticality": "medium", "data_classification": "internal"},
				out,
			)
			if err != nil {
				t.Fatalf("render %s: %v", c.id, err)
			}
			if len(res.FilesWritten) != len(m.Files) {
				t.Fatalf("%s: expected %d files written, got %d", c.id, len(m.Files), len(res.FilesWritten))
			}
			for _, f := range res.FilesWritten {
				data, err := os.ReadFile(f)
				if err != nil {
					t.Fatalf("read %s: %v", f, err)
				}
				if strings.Contains(string(data), "{{") {
					t.Fatalf("%s contains unrendered template placeholders", f)
				}
			}
		})
	}
}
