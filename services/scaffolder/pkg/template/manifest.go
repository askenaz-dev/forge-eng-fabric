// Package template loads, validates, and renders Forge repo template
// manifests as defined by the `repo-template-catalog` spec.
package template

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type ParameterSpec struct {
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Default     any      `yaml:"default"`
	Pattern     string   `yaml:"pattern"`
	Enum        []string `yaml:"enum"`
}

type FileSpec struct {
	Path     string `yaml:"path"`
	Template string `yaml:"template"`
}

type Manifest struct {
	ID                   string                   `yaml:"id"`
	Version              string                   `yaml:"version"`
	Description          string                   `yaml:"description"`
	Category             string                   `yaml:"category"`
	Parameters           map[string]ParameterSpec `yaml:"parameters"`
	PreHooks             []string                 `yaml:"pre_hooks"`
	PostHooks            []string                 `yaml:"post_hooks"`
	Files                []FileSpec               `yaml:"files"`
	RequiredCapabilities []string                 `yaml:"required_capabilities"`
}

func Load(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer f.Close()
	return Decode(f)
}

func Decode(r io.Reader) (*Manifest, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate ensures the manifest itself is well-formed (does not validate
// runtime parameter values; see `ValidateParams`).
func (m *Manifest) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("manifest: id is required")
	}
	if m.Version == "" {
		return fmt.Errorf("manifest: version is required")
	}
	if len(m.Files) == 0 {
		return fmt.Errorf("manifest: at least one file is required")
	}
	for i, f := range m.Files {
		if f.Path == "" {
			return fmt.Errorf("manifest: files[%d].path is required", i)
		}
		if strings.HasPrefix(f.Path, "/") || strings.Contains(f.Path, "..") {
			return fmt.Errorf("manifest: files[%d].path must be relative and not escape", i)
		}
	}
	for name, p := range m.Parameters {
		if p.Pattern != "" {
			if _, err := regexp.Compile(p.Pattern); err != nil {
				return fmt.Errorf("manifest: parameter %q has invalid pattern: %w", name, err)
			}
		}
	}
	return nil
}

// ValidateParams checks the supplied params against the manifest's parameter
// declarations: required-ness, enum membership, and regex pattern.
// Returns a non-nil error and the params map filled with defaults when
// validation succeeds.
func (m *Manifest) ValidateParams(params map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for k, v := range params {
		out[k] = v
	}
	for name, spec := range m.Parameters {
		val, present := out[name]
		if !present {
			if spec.Required {
				return nil, fmt.Errorf("missing required parameter: %s", name)
			}
			if spec.Default != nil {
				out[name] = spec.Default
				val = spec.Default
				present = true
			}
		}
		if !present {
			continue
		}
		if spec.Type == "string" {
			s, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("parameter %s: expected string, got %T", name, val)
			}
			if spec.Pattern != "" {
				re := regexp.MustCompile(spec.Pattern)
				if !re.MatchString(s) {
					return nil, fmt.Errorf("parameter %s: value %q does not match pattern %q", name, s, spec.Pattern)
				}
			}
			if len(spec.Enum) > 0 {
				ok := false
				for _, e := range spec.Enum {
					if e == s {
						ok = true
						break
					}
				}
				if !ok {
					return nil, fmt.Errorf("parameter %s: value %q not in enum %v", name, s, spec.Enum)
				}
			}
		}
	}
	return out, nil
}
