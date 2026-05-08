package template

import (
	"path/filepath"
	"strings"
	"testing"
)

const minimalManifest = `
id: tpl-min
version: 1.0.0
description: minimal
parameters:
  name:
    type: string
    required: true
files:
  - path: hello.txt
    template: "hello {{ .name }}"
`

func TestDecodeAndValidate(t *testing.T) {
	m, err := Decode(strings.NewReader(minimalManifest))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m.ID != "tpl-min" || m.Version != "1.0.0" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestValidateRejectsEscapingPath(t *testing.T) {
	src := `
id: bad
version: 1.0.0
files:
  - path: ../escape.txt
    template: x
`
	if _, err := Decode(strings.NewReader(src)); err == nil {
		t.Fatal("expected error for escaping path")
	}
}

func TestValidateParamsRequiredAndDefaults(t *testing.T) {
	src := `
id: tpl
version: 1.0.0
parameters:
  name:
    type: string
    required: true
  criticality:
    type: string
    required: false
    default: medium
    enum: [low, medium, high, critical]
files:
  - path: x
    template: x
`
	m, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, err := m.ValidateParams(map[string]any{}); err == nil {
		t.Fatal("expected missing required parameter error")
	}
	out, err := m.ValidateParams(map[string]any{"name": "svc-a"})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if out["criticality"] != "medium" {
		t.Fatalf("default not applied: got %v", out["criticality"])
	}
}

func TestValidateParamsEnumViolation(t *testing.T) {
	src := `
id: tpl
version: 1.0.0
parameters:
  level:
    type: string
    required: true
    enum: [low, medium]
files:
  - path: x
    template: x
`
	m, _ := Decode(strings.NewReader(src))
	if _, err := m.ValidateParams(map[string]any{"level": "extreme"}); err == nil {
		t.Fatal("expected enum violation")
	}
}

func TestValidateParamsPattern(t *testing.T) {
	src := `
id: tpl
version: 1.0.0
parameters:
  name:
    type: string
    required: true
    pattern: "^[a-z]+$"
files:
  - path: x
    template: x
`
	m, _ := Decode(strings.NewReader(src))
	if _, err := m.ValidateParams(map[string]any{"name": "Has-Caps"}); err == nil {
		t.Fatal("expected pattern mismatch")
	}
	if _, err := m.ValidateParams(map[string]any{"name": "ok"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestRenderProducesFilesWithWorkspaceData(t *testing.T) {
	m, err := Decode(strings.NewReader(minimalManifest))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	out := t.TempDir()
	res, err := m.Render(
		map[string]any{"name": "svc-foo"},
		map[string]any{"owner": "@team-a", "criticality": "high", "data_classification": "internal"},
		out,
	)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(res.FilesWritten) != 1 {
		t.Fatalf("expected 1 file, got %d", len(res.FilesWritten))
	}
	if filepath.Base(res.FilesWritten[0]) != "hello.txt" {
		t.Fatalf("unexpected filename: %s", res.FilesWritten[0])
	}
	if res.EffectiveParams["criticality"] != "high" {
		t.Fatalf("workspace data not merged: %+v", res.EffectiveParams)
	}
}

func TestRenderRejectsMissingRequired(t *testing.T) {
	m, err := Decode(strings.NewReader(minimalManifest))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	out := t.TempDir()
	if _, err := m.Render(map[string]any{}, map[string]any{}, out); err == nil {
		t.Fatal("expected error")
	}
}
