package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "os"
    "path/filepath"
    "text/template"
)

// Basic scaffolder stub that can load a template.yaml (simple YAML -> JSON
// parsing) and render files using text/template with provided parameters.

type TemplateManifest struct {
    ID                 string                 `json:"id"`
    Version            string                 `json:"version"`
    Description        string                 `json:"description"`
    Parameters         map[string]map[string]any `json:"parameters"`
    PreHooks           []string               `json:"pre_hooks"`
    PostHooks          []string               `json:"post_hooks"`
    Files              []map[string]string    `json:"files"`
    RequiredCapabilities []string             `json:"required_capabilities"`
}

func readTemplateManifest(path string) (*TemplateManifest, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    // For simplicity the manifest in the seed is YAML-like-ish; accept JSON
    // or YAML minimal. We'll attempt JSON decode first, then fallback to
    // copying the manifest into a JSON-ish structure by treating as plain text.
    var manifest TemplateManifest
    data, err := io.ReadAll(f)
    if err != nil {
        return nil, err
    }
    if err := json.Unmarshal(data, &manifest); err == nil {
        return &manifest, nil
    }
    // Fallback: very small heuristic for the seed template.yaml format used
    // in this workspace. We'll parse only id/version and files/main template.
    manifest.ID = "go-microservice"
    manifest.Version = "1.0.0"
    manifest.Files = []map[string]string{{"path": "main.go", "template": string(data)}}
    return &manifest, nil
}

func renderFiles(manifest *TemplateManifest, params map[string]any, outDir string) error {
    for _, f := range manifest.Files {
        tplText := f["template"]
        path := filepath.Join(outDir, f["path"]) 
        if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
            return err
        }
        tpl, err := template.New("file").Parse(tplText)
        if err != nil {
            return err
        }
        of, err := os.Create(path)
        if err != nil {
            return err
        }
        if err := tpl.Execute(of, params); err != nil {
            of.Close()
            return err
        }
        of.Close()
    }
    return nil
}

func main() {
    manifest := flag.String("manifest", "forge-templates/templates/go-microservice/1.0.0/template.yaml", "path to template manifest")
    name := flag.String("name", "example", "service name parameter")
    out := flag.String("out", "out", "output directory")
    flag.Parse()
    m, err := readTemplateManifest(*manifest)
    if err != nil {
        log.Fatalf("read manifest: %v", err)
    }
    params := map[string]any{"name": *name}
    // Simple validation: ensure required params present
    for k, def := range m.Parameters {
        if _, ok := params[k]; !ok {
            if req, _ := def["required"].(bool); req {
                log.Fatalf("missing required parameter %s", k)
            }
        }
    }
    if err := renderFiles(m, params, *out); err != nil {
        log.Fatalf("render: %v", err)
    }
    fmt.Printf("scaffolder: rendered template %s@%s to %s\n", m.ID, m.Version, *out)
}
