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
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

// design-system-catalog (Section 9): `forge design-system ...` commands let
// developers browse the catalog, preview a template, and install or swap a
// design system on an App. Resolution happens against the Registry and the
// application service.

func designSystemCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "design-system", Short: "Browse, preview, install and swap Design Systems"}
	cmd.AddCommand(dsListCmd(), dsPreviewCmd(), dsInstallCmd(), dsSwapCmd())
	return cmd
}

func dsListCmd() *cobra.Command {
	var registry, tenant string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Design Systems visible to the caller",
		RunE: func(cmd *cobra.Command, _ []string) error {
			base := registryBase(registry)
			body, err := registryGet(cmd.Context(), base+"/v1/design-systems")
			if err != nil {
				return err
			}
			var entries []map[string]any
			if err := json.Unmarshal(body, &entries); err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Println("(no design systems in catalog)")
				return nil
			}
			for _, e := range entries {
				if tenant != "" && e["tenant_id"] != tenant && e["visibility"] != "tenant_global" {
					continue
				}
				name := stringOf(e["name"])
				version := stringOf(e["version"])
				lifecycle := stringOf(e["lifecycle_state"])
				visibility := stringOf(e["visibility"])
				fmt.Printf("%s@%s\t%s\t%s\n", name, version, lifecycle, visibility)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&registry, "registry", "", "Registry base URL (env: FORGE_REGISTRY_URL)")
	cmd.Flags().StringVar(&tenant, "tenant", "", "Filter results to a specific tenant id")
	return cmd
}

func dsPreviewCmd() *cobra.Command {
	var registry string
	cmd := &cobra.Command{
		Use:   "preview <ref>",
		Short: "Render the sample composition for a Design System in your browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base := registryBase(registry)
			ref := args[0]
			body, err := registryGet(cmd.Context(), base+"/v1/design-systems/"+url.PathEscape(ref))
			if err != nil {
				return err
			}
			var entry map[string]any
			if err := json.Unmarshal(body, &entry); err != nil {
				return err
			}
			manifest, _ := entry["manifest"].(map[string]any)
			screenshots, _ := manifest["screenshots"].(map[string]any)
			lightURL, _ := screenshots["light"].(string)
			darkURL, _ := screenshots["dark"].(string)
			useCase, _ := manifest["use_case"].(string)
			html := fmt.Sprintf(`<!doctype html>
<html><head><title>%[1]s</title>
<style>body{font-family:system-ui;margin:24px;color:#222} .grid{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin-top:16px} img{max-width:100%%;border:1px solid #ddd;border-radius:6px}</style>
</head><body>
<h1>%[1]s</h1><p>%[2]s</p>
<div class="grid">
  <figure><img src="%[3]s" alt="light"><figcaption>Light</figcaption></figure>
  <figure><img src="%[4]s" alt="dark"><figcaption>Dark</figcaption></figure>
</div>
</body></html>`, stringOf(entry["name"]), useCase, lightURL, darkURL)
			tmp, err := os.CreateTemp("", "forge-ds-preview-*.html")
			if err != nil {
				return err
			}
			defer tmp.Close()
			if _, err := tmp.WriteString(html); err != nil {
				return err
			}
			path := tmp.Name()
			fmt.Printf("opened preview: %s\n", path)
			return openInBrowser(path)
		},
	}
	cmd.Flags().StringVar(&registry, "registry", "", "Registry base URL")
	return cmd
}

func dsInstallCmd() *cobra.Command {
	var application, appID, ref, reason string
	cmd := &cobra.Command{
		Use:   "install <ref> --app <id>",
		Short: "Install a Design System on an App (opens a swap PR)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if appID == "" {
				return errors.New("--app is required")
			}
			ref = args[0]
			base := applicationBase(application)
			payload := map[string]any{"target_ref": ref}
			if reason != "" {
				payload["reason"] = reason
			}
			return appPost(cmd.Context(), base+"/v1/apps/"+url.PathEscape(appID)+"/design-system:swap", payload)
		},
	}
	cmd.Flags().StringVar(&application, "application", "", "application service base URL")
	cmd.Flags().StringVar(&appID, "app", "", "target App id")
	cmd.Flags().StringVar(&reason, "reason", "", "optional reason recorded on the swap PR")
	return cmd
}

func dsSwapCmd() *cobra.Command {
	var application, to, reason string
	cmd := &cobra.Command{
		Use:   "swap <app> --to <ref>",
		Short: "Swap the Design System on an App (convenience wrapper for install)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return errors.New("--to is required")
			}
			appID := args[0]
			base := applicationBase(application)
			payload := map[string]any{"target_ref": to}
			if reason != "" {
				payload["reason"] = reason
			}
			return appPost(cmd.Context(), base+"/v1/apps/"+url.PathEscape(appID)+"/design-system:swap", payload)
		},
	}
	cmd.Flags().StringVar(&application, "application", "", "application service base URL")
	cmd.Flags().StringVar(&to, "to", "", "target design_system_ref")
	cmd.Flags().StringVar(&reason, "reason", "", "optional reason recorded on the swap PR")
	return cmd
}

func registryBase(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("FORGE_REGISTRY_URL"); v != "" {
		return v
	}
	return "http://localhost:8082"
}

func applicationBase(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("FORGE_APPLICATION_URL"); v != "" {
		return v
	}
	return "http://localhost:8095"
}

func registryGet(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s: %s — %s", u, resp.Status, string(body))
	}
	return body, nil
}

func appPost(ctx context.Context, u string, payload map[string]any) error {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
	req.Body = io.NopCloser(bytesReader(body))
	req.Header.Set("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s: %s — %s", u, resp.Status, string(respBody))
	}
	fmt.Println(string(respBody))
	return nil
}

func stringOf(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func bytesReader(b []byte) *bytesReaderT { return &bytesReaderT{b: b} }

type bytesReaderT struct {
	b []byte
	i int
}

func (b *bytesReaderT) Read(p []byte) (int, error) {
	if b.i >= len(b.b) {
		return 0, io.EOF
	}
	n := copy(p, b.b[b.i:])
	b.i += n
	return n, nil
}

func openInBrowser(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
