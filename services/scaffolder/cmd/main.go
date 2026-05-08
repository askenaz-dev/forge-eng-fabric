package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	tpl "github.com/forge-eng-fabric/services/scaffolder/pkg/template"
)

// Forge scaffolder CLI: renders a versioned repo template into a target
// directory. Workspace metadata (criticality, data_classification, owners)
// is supplied via -workspace-json so the calling service does not need
// shell-escaping for nested values.
func main() {
	manifest := flag.String("manifest", "forge-templates/templates/go-microservice/1.0.0/template.yaml", "path to template manifest")
	out := flag.String("out", "out", "output directory")
	paramJSON := flag.String("params", "{}", "JSON object with template parameters")
	workspaceJSON := flag.String("workspace-json", "{}", "JSON object with workspace metadata to merge into params")
	emitJSON := flag.Bool("json", false, "emit RenderResult as JSON on stdout")
	flag.Parse()

	m, err := tpl.Load(*manifest)
	if err != nil {
		log.Fatalf("manifest: %v", err)
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(*paramJSON), &params); err != nil {
		log.Fatalf("parse params: %v", err)
	}
	var workspace map[string]any
	if err := json.Unmarshal([]byte(*workspaceJSON), &workspace); err != nil {
		log.Fatalf("parse workspace: %v", err)
	}
	res, err := m.Render(params, workspace, *out)
	if err != nil {
		log.Fatalf("render: %v", err)
	}
	if *emitJSON {
		out, _ := json.Marshal(res)
		fmt.Println(string(out))
		return
	}
	fmt.Printf("scaffolder: rendered %s@%s -> %s (%d files)\n", m.ID, m.Version, *out, len(res.FilesWritten))
	for _, f := range res.FilesWritten {
		// Use forward slashes for portable display
		fmt.Println("  ", strings.ReplaceAll(f, string(os.PathSeparator), "/"))
	}
}
