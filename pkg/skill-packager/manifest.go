package skillpackager

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{1,63}$`)

// renderManifest builds the SKILL.md content from the spec's metadata.
// Format: YAML front-matter delimited by `---` followed by the markdown body.
// Keys are emitted in a fixed order so the output is byte-stable.
func renderManifest(spec Spec) ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteString("---\n")
	buf.WriteString("name: ")
	buf.WriteString(yamlString(spec.Name))
	buf.WriteString("\n")
	buf.WriteString("description: ")
	buf.WriteString(yamlString(strings.TrimSpace(spec.Description)))
	buf.WriteString("\n")
	if len(spec.MCPDependencies) > 0 {
		buf.WriteString("mcp:\n")
		for _, m := range spec.MCPDependencies {
			if strings.TrimSpace(m) == "" {
				return nil, fmt.Errorf("empty MCP dependency entry")
			}
			buf.WriteString("  - ")
			buf.WriteString(yamlString(m))
			buf.WriteString("\n")
		}
	}
	buf.WriteString("---\n")
	body := strings.TrimRight(spec.Body, "\n")
	if body != "" {
		buf.WriteString("\n")
		buf.WriteString(body)
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

// yamlString quotes a string for YAML. We always quote with double quotes and
// escape `"` + `\` — sufficient for the field shapes we accept (single-line,
// no unicode control chars) and stable across Go versions.
func yamlString(s string) string {
	needsQuote := s == "" || strings.ContainsAny(s, ":#[]{}&*!|>'\"%@`") || strings.HasPrefix(s, "- ") || strings.ContainsAny(s, "\n\t")
	if !needsQuote {
		return s
	}
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
