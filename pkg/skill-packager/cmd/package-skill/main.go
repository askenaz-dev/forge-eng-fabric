// package-skill is the developer entrypoint behind `make package-skill`. It
// reads a YAML skill spec, optionally fetches its references, and writes a
// deterministic Agent Skills bundle (.tar.zst) to disk plus a sidecar
// `<out>.digest` containing the sha256 the registry will accept.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	skillpackager "github.com/forge-eng-fabric/pkg/skill-packager"
)

// inputSpec is the on-disk format. We accept JSON or YAML-ish JSON here to
// avoid a YAML dependency; CI pipelines typically render this to JSON first.
type inputSpec struct {
	Name            string                     `json:"name"`
	Description     string                     `json:"description"`
	Body            string                     `json:"body"`
	MCPDependencies []string                   `json:"mcp,omitempty"`
	Files           []inputFile                `json:"files,omitempty"`
	References      []skillpackager.Reference  `json:"references,omitempty"`
}

type inputFile struct {
	Path       string `json:"path"`
	Path_      string `json:"-"`
	Body       string `json:"body"`
	BodyB64    string `json:"body_b64,omitempty"`
	Executable bool   `json:"executable,omitempty"`
}

func main() {
	var (
		specPath = flag.String("spec", "", "path to skill JSON spec")
		outPath  = flag.String("out", "", "path to write the .tar.zst bundle")
	)
	flag.Parse()
	if *specPath == "" || *outPath == "" {
		fmt.Fprintln(os.Stderr, "usage: package-skill -spec <spec.json> -out <bundle.tar.zst>")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*specPath)
	if err != nil {
		fail(err)
	}
	var in inputSpec
	if err := json.Unmarshal(raw, &in); err != nil {
		fail(fmt.Errorf("parse spec: %w", err))
	}
	files := make([]skillpackager.File, 0, len(in.Files))
	for _, f := range in.Files {
		files = append(files, skillpackager.File{Path: f.Path, Body: []byte(f.Body), Executable: f.Executable})
	}
	if len(in.References) > 0 {
		refs, err := skillpackager.Embed(context.Background(), in.References, nil)
		if err != nil {
			fail(err)
		}
		files = append(files, refs...)
	}
	res, err := skillpackager.Package(skillpackager.Spec{
		Name:            in.Name,
		Description:     in.Description,
		Body:            in.Body,
		MCPDependencies: in.MCPDependencies,
		Files:           files,
	}, skillpackager.DefaultLimits)
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*outPath, res.Bytes, 0o644); err != nil {
		fail(err)
	}
	if err := os.WriteFile(*outPath+".digest", []byte(res.Digest+"\n"), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("packaged %s (%d bytes) -> %s\n", res.Digest, res.Size, *outPath)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "package-skill:", err.Error())
	os.Exit(1)
}
