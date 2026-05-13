// Package clients holds the client-detection table — the data-not-code list
// of agentic clients the CLI supports. Each entry resolves a base directory
// where Agent Skills bundles must be unpacked and an MCP config file the
// installer mutates when a skill declares MCP dependencies.
package clients

import (
	"os"
	"path/filepath"
	"runtime"
)

// Client describes one supported agentic client.
type Client struct {
	// ID is the kebab-case identifier used by `forge skills install --client`.
	ID string `json:"id"`
	// Name is the human-readable label.
	Name string `json:"name"`
	// SkillsDir returns the absolute directory under which skill bundles
	// should be unpacked, given the user's home directory.
	SkillsDir func(home string) string `json:"-"`
	// MCPConfig returns the path to the client's MCP config JSON (empty
	// when the client does not expose one we can edit).
	MCPConfig func(home string) string `json:"-"`
	// DetectMarker is a relative path under home whose existence implies
	// the client is installed; used by auto-detection.
	DetectMarker string `json:"detect_marker"`
}

// Registry returns every supported client. New clients are added here.
func Registry() []Client {
	return []Client{
		{
			ID:           "claude-code",
			Name:         "Claude Code",
			SkillsDir:    func(h string) string { return filepath.Join(h, ".claude", "skills") },
			MCPConfig:    func(h string) string { return filepath.Join(h, ".claude", "mcp.json") },
			DetectMarker: ".claude",
		},
		{
			ID:        "claude-desktop",
			Name:      "Claude desktop",
			SkillsDir: claudeDesktopDir,
			MCPConfig: claudeDesktopMCP,
			DetectMarker: claudeDesktopMarker(),
		},
		{
			ID:           "copilot",
			Name:         "GitHub Copilot",
			SkillsDir:    func(h string) string { return filepath.Join(h, ".config", "github-copilot", "skills") },
			MCPConfig:    func(h string) string { return filepath.Join(h, ".config", "github-copilot", "mcp.json") },
			DetectMarker: ".config/github-copilot",
		},
		{
			ID:           "codex",
			Name:         "OpenAI Codex CLI",
			SkillsDir:    func(h string) string { return filepath.Join(h, ".codex", "skills") },
			MCPConfig:    func(h string) string { return filepath.Join(h, ".codex", "mcp.json") },
			DetectMarker: ".codex",
		},
		{
			ID:           "cursor",
			Name:         "Cursor",
			SkillsDir:    func(h string) string { return filepath.Join(h, ".cursor", "skills") },
			MCPConfig:    func(h string) string { return filepath.Join(h, ".cursor", "mcp.json") },
			DetectMarker: ".cursor",
		},
		{
			ID:           "gemini-cli",
			Name:         "Gemini CLI",
			SkillsDir:    func(h string) string { return filepath.Join(h, ".gemini", "skills") },
			MCPConfig:    func(h string) string { return filepath.Join(h, ".gemini", "mcp.json") },
			DetectMarker: ".gemini",
		},
		{
			ID:           "openhands",
			Name:         "OpenHands",
			SkillsDir:    func(h string) string { return filepath.Join(h, ".openhands", "skills") },
			MCPConfig:    func(h string) string { return filepath.Join(h, ".openhands", "mcp.json") },
			DetectMarker: ".openhands",
		},
		{
			ID:           "opencode",
			Name:         "OpenCode",
			SkillsDir:    func(h string) string { return filepath.Join(h, ".opencode", "skills") },
			MCPConfig:    func(h string) string { return filepath.Join(h, ".opencode", "mcp.json") },
			DetectMarker: ".opencode",
		},
		{
			ID:           "vscode",
			Name:         "VS Code (workspace)",
			SkillsDir:    func(h string) string { return filepath.Join(".vscode", "skills") },
			MCPConfig:    func(h string) string { return filepath.Join(".vscode", "mcp.json") },
			DetectMarker: ".vscode",
		},
		{
			ID:           "generic",
			Name:         "Generic / unknown",
			SkillsDir:    func(h string) string { return filepath.Join(h, ".agentskills") },
			MCPConfig:    func(h string) string { return "" },
			DetectMarker: "",
		},
	}
}

// ByID returns the client with the given id, or nil.
func ByID(id string) *Client {
	for _, c := range Registry() {
		if c.ID == id {
			return &c
		}
	}
	return nil
}

// Detect picks the active client based on which marker directories exist on
// the developer's machine. Returns the generic client if none match.
func Detect() Client {
	home, err := os.UserHomeDir()
	if err != nil {
		return *ByID("generic")
	}
	for _, c := range Registry() {
		if c.ID == "generic" || c.DetectMarker == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(home, c.DetectMarker)); err == nil {
			return c
		}
	}
	return *ByID("generic")
}

// claudeDesktopDir resolves Claude desktop's skills directory per platform.
func claudeDesktopDir(home string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "skills")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Claude", "skills")
	default:
		return filepath.Join(home, ".config", "Claude", "skills")
	}
}

// claudeDesktopMCP resolves the Claude desktop MCP config file per platform.
func claudeDesktopMCP(home string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Claude", "claude_desktop_config.json")
	default:
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	}
}

// claudeDesktopMarker is the relative-to-home path used by Detect on the
// caller's OS.
func claudeDesktopMarker() string {
	switch runtime.GOOS {
	case "darwin":
		return "Library/Application Support/Claude"
	case "windows":
		// Marker resolution under home is best-effort on Windows where the
		// real path lives under %APPDATA%.
		return "AppData/Roaming/Claude"
	default:
		return ".config/Claude"
	}
}
