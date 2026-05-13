// Forge developer CLI: install governed Forge skills into the developer's
// agentic client (Claude Code, Copilot, Codex, Cursor, …). See
// openspec/changes/add-developer-skill-gateway for the spec.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"

	"github.com/askenaz-dev/forge-eng-fabric/cli/forge/internal/clients"
	"github.com/askenaz-dev/forge-eng-fabric/cli/forge/internal/gateway"
	"github.com/askenaz-dev/forge-eng-fabric/cli/forge/internal/mcpwire"
	"github.com/askenaz-dev/forge-eng-fabric/cli/forge/internal/unpack"
)

// Build-time variables filled in by goreleaser / `go build -ldflags`.
var (
	version = "dev"
	commit  = "none"
	signed  = "unsigned"
)

const keyringService = "forge"

func main() {
	root := &cobra.Command{
		Use:           "forge",
		Short:         "Forge developer CLI — install governed Forge skills into your agent",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s (commit %s, signed-by %s)", version, commit, signed),
	}

	root.AddCommand(loginCmd(), logoutCmd(), skillsCmd(), clientsCmd(), configCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "forge:", err.Error())
		os.Exit(1)
	}
}

// --- login / logout ---------------------------------------------------

func loginCmd() *cobra.Command {
	var gw string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against a Forge skill gateway via OIDC device-code",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if gw == "" {
				gw = os.Getenv("FORGE_GATEWAY")
			}
			if gw == "" {
				return errors.New("--gateway (or FORGE_GATEWAY) is required")
			}
			return runLogin(cmd.Context(), gw)
		},
	}
	cmd.Flags().StringVar(&gw, "gateway", "", "URL of the Forge skill gateway (e.g. https://acme.forge.dev)")
	return cmd
}

func runLogin(ctx context.Context, gw string) error {
	body, err := postJSON(ctx, gw+"/v1/gateway/auth/device", map[string]any{})
	if err != nil {
		return err
	}
	var dev struct {
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
		DeviceCode      string `json:"device_code"`
	}
	_ = json.Unmarshal(body, &dev)
	if dev.VerificationURI == "" {
		return fmt.Errorf("gateway device-auth not configured yet (501) — wire keycloak per design.md decision 6")
	}
	fmt.Printf("Open %s and enter %s\n", dev.VerificationURI, dev.UserCode)
	interval := dev.Interval
	if interval <= 0 {
		interval = 5
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(interval) * time.Second):
		}
		raw, err := postJSON(ctx, gw+"/v1/gateway/auth/token", map[string]any{"device_code": dev.DeviceCode})
		if err == nil {
			var tok struct {
				RefreshToken string `json:"refresh_token"`
				AccessToken  string `json:"access_token"`
			}
			_ = json.Unmarshal(raw, &tok)
			if tok.RefreshToken != "" {
				if err := keyring.Set(keyringService, gw, tok.RefreshToken); err != nil {
					return fmt.Errorf("save token in OS keystore: %w", err)
				}
				fmt.Println("logged in. Token stored in the OS keystore.")
				return nil
			}
		}
	}
}

func logoutCmd() *cobra.Command {
	var gw string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Forget the saved Forge token",
		RunE: func(_ *cobra.Command, _ []string) error {
			if gw == "" {
				gw = os.Getenv("FORGE_GATEWAY")
			}
			if gw == "" {
				return errors.New("--gateway (or FORGE_GATEWAY) is required")
			}
			if err := keyring.Delete(keyringService, gw); err != nil {
				return err
			}
			fmt.Println("logged out")
			return nil
		},
	}
	cmd.Flags().StringVar(&gw, "gateway", "", "gateway URL")
	return cmd
}

func resolveToken(gw string) string {
	if t := os.Getenv("FORGE_TOKEN"); t != "" {
		return t
	}
	if t, err := keyring.Get(keyringService, gw); err == nil {
		return t
	}
	return ""
}

// --- skills sub-tree --------------------------------------------------

func skillsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "skills", Short: "Manage installed Forge skills"}
	cmd.AddCommand(skillsListCmd(), skillsSearchCmd(), skillsInstallCmd(), skillsRemoveCmd(), skillsUpdateCmd(), skillsStatusCmd())
	return cmd
}

func skillsListCmd() *cobra.Command {
	var gw string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installable skills from the gateway",
		RunE: func(cmd *cobra.Command, _ []string) error {
			gw := pickGateway(gw)
			c := gateway.New(gw, resolveToken(gw))
			assets, err := c.List(cmd.Context(), "")
			if err != nil {
				return err
			}
			active := clients.Detect()
			fmt.Printf("active client: %s\n", active.Name)
			for _, a := range assets {
				marker := "  "
				if isInstalled(active, a.Name) {
					marker = "✓ "
				}
				fmt.Printf("%s%s@%s\t%s\t%s\n", marker, a.Name, a.Version, a.Type, a.TrustLevel)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&gw, "gateway", "", "gateway URL")
	return cmd
}

func skillsSearchCmd() *cobra.Command {
	var gw, query string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search installable skills",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query = args[0]
			c := gateway.New(pickGateway(gw), resolveToken(pickGateway(gw)))
			assets, err := c.List(cmd.Context(), query)
			if err != nil {
				return err
			}
			for _, a := range assets {
				if !strings.Contains(strings.ToLower(a.Name+" "+a.Description), strings.ToLower(query)) {
					continue
				}
				fmt.Printf("%s@%s\t%s\t%s\n", a.Name, a.Version, a.TrustLevel, a.Description)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&gw, "gateway", "", "gateway URL")
	return cmd
}

func skillsInstallCmd() *cobra.Command {
	var gw, clientID string
	cmd := &cobra.Command{
		Use:   "install <name>[@<version>]",
		Short: "Install a skill into the active client",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, version := splitNameVersion(args[0])
			c := gateway.New(pickGateway(gw), resolveToken(pickGateway(gw)))
			assets, err := c.List(cmd.Context(), "")
			if err != nil {
				return err
			}
			var target *gateway.Asset
			for i, a := range assets {
				if a.Name == name && (version == "" || a.Version == version) {
					target = &assets[i]
					break
				}
			}
			if target == nil {
				return fmt.Errorf("skill %s not found in your workspace", args[0])
			}
			pkg, err := c.Download(cmd.Context(), target.ID, target.Version)
			if err != nil {
				return err
			}
			active := pickClient(clientID)
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			destRoot := active.SkillsDir(home)
			folder, err := unpack.Into(pkg.Bytes, destRoot)
			if err != nil {
				return err
			}
			fmt.Printf("installed %s@%s -> %s\n", target.Name, target.Version, filepath.Join(destRoot, folder))
			// Wire MCP if declared.
			if mcps := readSkillMCPs(filepath.Join(destRoot, folder, "SKILL.md")); len(mcps) > 0 {
				for _, mcpID := range mcps {
					gwURL := pickGateway(gw)
					if _, err := mcpwire.EnsureEntry(active.MCPConfig(home), active.ID, mcpwire.Entry{
						AssetID:    mcpID,
						AssetName:  mcpID,
						GatewayURL: gwURL + "/v1/gateway/mcp/" + url.PathEscape(mcpID),
					}); err != nil {
						return fmt.Errorf("wire mcp %s: %w", mcpID, err)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&gw, "gateway", "", "gateway URL")
	cmd.Flags().StringVar(&clientID, "client", "", "force a client id (claude-code, copilot, codex, cursor, ...)")
	return cmd
}

func skillsRemoveCmd() *cobra.Command {
	var clientID string
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed skill from the active client",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			active := pickClient(clientID)
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			dir := filepath.Join(active.SkillsDir(home), args[0])
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			_, _ = mcpwire.RemoveEntry(active.MCPConfig(home), active.ID, args[0])
			fmt.Println("removed", dir)
			return nil
		},
	}
	cmd.Flags().StringVar(&clientID, "client", "", "force a client id")
	return cmd
}

func skillsUpdateCmd() *cobra.Command {
	var gw, clientID string
	cmd := &cobra.Command{
		Use:   "update [<name>]",
		Short: "Upgrade installed skills to the latest approved version",
		RunE: func(cmd *cobra.Command, args []string) error {
			active := pickClient(clientID)
			home, _ := os.UserHomeDir()
			entries, err := os.ReadDir(active.SkillsDir(home))
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Println("no skills installed")
					return nil
				}
				return err
			}
			c := gateway.New(pickGateway(gw), resolveToken(pickGateway(gw)))
			latest, err := c.List(cmd.Context(), "")
			if err != nil {
				return err
			}
			byName := map[string]gateway.Asset{}
			for _, a := range latest {
				byName[a.Name] = a
			}
			for _, e := range entries {
				if len(args) > 0 && e.Name() != args[0] {
					continue
				}
				if a, ok := byName[e.Name()]; ok {
					fmt.Printf("upgrading %s -> %s\n", e.Name(), a.Version)
					pkg, err := c.Download(cmd.Context(), a.ID, a.Version)
					if err != nil {
						return err
					}
					if _, err := unpack.Into(pkg.Bytes, active.SkillsDir(home)); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&gw, "gateway", "", "gateway URL")
	cmd.Flags().StringVar(&clientID, "client", "", "force a client id")
	return cmd
}

func skillsStatusCmd() *cobra.Command {
	var clientID string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show installed skills per client",
		RunE: func(_ *cobra.Command, _ []string) error {
			home, _ := os.UserHomeDir()
			for _, c := range clients.Registry() {
				if clientID != "" && c.ID != clientID {
					continue
				}
				entries, err := os.ReadDir(c.SkillsDir(home))
				if err != nil {
					continue
				}
				if len(entries) == 0 {
					continue
				}
				fmt.Printf("# %s\n", c.Name)
				for _, e := range entries {
					fmt.Printf("  %s\n", e.Name())
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&clientID, "client", "", "limit to one client id")
	return cmd
}

// --- clients ----------------------------------------------------------

func clientsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clients",
		Short: "List supported agentic clients and their install paths",
		RunE: func(_ *cobra.Command, _ []string) error {
			home, _ := os.UserHomeDir()
			for _, c := range clients.Registry() {
				fmt.Printf("%s\t%s\t%s\n", c.ID, c.Name, c.SkillsDir(home))
			}
			return nil
		},
	}
}

// --- config (telemetry) -----------------------------------------------

func configCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Configure the CLI"}
	cmd.AddCommand(&cobra.Command{
		Use:  "set <key> <value>",
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			path := filepath.Join(home, ".forge", "config.json")
			conf := map[string]any{}
			if data, err := os.ReadFile(path); err == nil {
				_ = json.Unmarshal(data, &conf)
			}
			conf[args[0]] = args[1]
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			body, _ := json.MarshalIndent(conf, "", "  ")
			return os.WriteFile(path, append(body, '\n'), 0o600)
		},
	})
	return cmd
}

// --- helpers ----------------------------------------------------------

func pickGateway(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("FORGE_GATEWAY"); v != "" {
		return v
	}
	return "http://localhost:8120"
}

func pickClient(id string) clients.Client {
	if id != "" {
		if c := clients.ByID(id); c != nil {
			return *c
		}
		fmt.Fprintf(os.Stderr, "warning: unknown client %q, using generic\n", id)
		return *clients.ByID("generic")
	}
	return clients.Detect()
}

func splitNameVersion(s string) (string, string) {
	if i := strings.Index(s, "@"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func isInstalled(c clients.Client, name string) bool {
	home, _ := os.UserHomeDir()
	_, err := os.Stat(filepath.Join(c.SkillsDir(home), name))
	return err == nil
}

func readSkillMCPs(skillMD string) []string {
	data, err := os.ReadFile(skillMD)
	if err != nil {
		return nil
	}
	body := string(data)
	if !strings.HasPrefix(body, "---") {
		return nil
	}
	end := strings.Index(body[3:], "---")
	if end < 0 {
		return nil
	}
	front := body[3 : 3+end]
	var out []string
	in := false
	for _, line := range strings.Split(front, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "mcp:") {
			in = true
			continue
		}
		if in {
			if strings.HasPrefix(trim, "- ") {
				out = append(out, strings.Trim(strings.TrimPrefix(trim, "- "), "\""))
				continue
			}
			if trim != "" && !strings.HasPrefix(line, " ") {
				break
			}
		}
	}
	return out
}

func postJSON(ctx context.Context, urlStr string, payload map[string]any) ([]byte, error) {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, strings.NewReader(string(body)))
	req.Header.Set("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s: %s", urlStr, resp.Status)
	}
	return raw, nil
}
