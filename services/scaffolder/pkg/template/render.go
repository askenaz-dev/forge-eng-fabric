package template

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"
)

// RenderResult is the outcome of a successful render — files written to disk
// and the parameters effectively used (after defaults).
type RenderResult struct {
	OutputDir   string         `json:"output_dir"`
	FilesWritten []string      `json:"files_written"`
	EffectiveParams map[string]any `json:"effective_params"`
	HookOutputs []HookOutput   `json:"hook_outputs"`
}

type HookOutput struct {
	Phase    string        `json:"phase"`
	Command  string        `json:"command"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration_ns"`
}

// Render writes all files declared in the manifest to outDir using the
// supplied parameters merged with workspace metadata. Pre/post hooks run
// inside `outDir` (sandbox = working directory only) and their captured
// output is returned.
func (m *Manifest) Render(params map[string]any, workspace map[string]any, outDir string) (*RenderResult, error) {
	merged, err := m.ValidateParams(mergeMaps(params, workspace))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir output: %w", err)
	}
	res := &RenderResult{OutputDir: outDir, EffectiveParams: merged}

	for _, h := range m.PreHooks {
		out, err := runHook("pre", h, outDir)
		res.HookOutputs = append(res.HookOutputs, out)
		if err != nil {
			return res, fmt.Errorf("pre_hook failed: %w", err)
		}
	}

	for _, f := range m.Files {
		written, err := renderFile(f, merged, outDir)
		if err != nil {
			return res, fmt.Errorf("render %s: %w", f.Path, err)
		}
		res.FilesWritten = append(res.FilesWritten, written)
	}

	for _, h := range m.PostHooks {
		out, err := runHook("post", h, outDir)
		res.HookOutputs = append(res.HookOutputs, out)
		if err != nil {
			return res, fmt.Errorf("post_hook failed: %w", err)
		}
	}
	return res, nil
}

func renderFile(f FileSpec, params map[string]any, outDir string) (string, error) {
	target := filepath.Join(outDir, f.Path)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	tpl, err := template.New(f.Path).Option("missingkey=zero").Parse(f.Template)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, params); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	return target, nil
}

func runHook(phase, command, workDir string) (HookOutput, error) {
	out := HookOutput{Phase: phase, Command: command}
	start := time.Now()
	// The hook executes in `workDir` only — that's the sandbox boundary.
	// We do not propagate environment beyond a minimal allowlist.
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = workDir
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=" + workDir}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	out.Stdout = stdout.String()
	out.Stderr = stderr.String()
	out.Duration = time.Since(start)
	if exitErr, ok := err.(*exec.ExitError); ok {
		out.ExitCode = exitErr.ExitCode()
	}
	return out, err
}

func mergeMaps(a, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		// b is workspace metadata — only fill if not already provided.
		if _, ok := out[k]; !ok {
			out[k] = v
		}
	}
	return out
}
