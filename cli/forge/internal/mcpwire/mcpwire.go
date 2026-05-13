// Package mcpwire idempotently inserts a Forge gateway MCP entry into the
// active client's MCP config file. All clients use a JSON object keyed by
// MCP server name; differences across clients (top-level key, casing,
// outer envelope) are handled per client id.
package mcpwire

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Entry is a single gateway MCP entry; "forge:" prefix is added by the writer.
type Entry struct {
	AssetID     string
	AssetName   string
	GatewayURL  string // e.g. https://acme.forge.dev/v1/gateway/mcp/<assetID>
}

// EnsureEntry mutates the configFile so it contains an MCP entry pointing at
// the gateway. Returns true when it modified the file.
func EnsureEntry(configFile string, clientID string, entry Entry) (bool, error) {
	if configFile == "" {
		return false, nil // client does not support MCP wiring
	}
	root, key, err := loadOrInit(configFile, clientID)
	if err != nil {
		return false, err
	}
	servers, ok := root[key].(map[string]any)
	if !ok || servers == nil {
		servers = map[string]any{}
	}
	name := "forge:" + entry.AssetName
	desired := map[string]any{
		"transport": "http",
		"url":       entry.GatewayURL,
		"headers": map[string]any{
			"Authorization": "Bearer ${FORGE_TOKEN}",
		},
		"forge_asset_id": entry.AssetID,
	}
	if existing, ok := servers[name].(map[string]any); ok && sameEntry(existing, desired) {
		return false, nil
	}
	servers[name] = desired
	root[key] = servers
	return true, writeJSON(configFile, root)
}

// RemoveEntry deletes the forge: namespaced entry for one asset, returning
// whether the file changed.
func RemoveEntry(configFile string, clientID string, assetName string) (bool, error) {
	if configFile == "" {
		return false, nil
	}
	if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	root, key, err := loadOrInit(configFile, clientID)
	if err != nil {
		return false, err
	}
	servers, ok := root[key].(map[string]any)
	if !ok {
		return false, nil
	}
	name := "forge:" + assetName
	if _, present := servers[name]; !present {
		return false, nil
	}
	delete(servers, name)
	root[key] = servers
	return true, writeJSON(configFile, root)
}

func loadOrInit(path, clientID string) (map[string]any, string, error) {
	root := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, "", fmt.Errorf("parse %s: %w", path, err)
		}
	}
	return root, mcpKey(clientID), nil
}

// mcpKey returns the top-level JSON key under which the client expects MCP
// servers to live. Claude Desktop uses "mcpServers"; Claude Code, Codex,
// Cursor, etc. use "servers"; future clients can extend this.
func mcpKey(clientID string) string {
	switch clientID {
	case "claude-desktop":
		return "mcpServers"
	default:
		return "servers"
	}
}

func sameEntry(a, b map[string]any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func writeJSON(path string, root map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
