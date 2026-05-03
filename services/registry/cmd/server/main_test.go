package main

import "testing"

func TestSemVerPattern(t *testing.T) {
	tests := map[string]bool{
		"0.1.0":    true,
		"1.0.0":    true,
		"10.20.30": true,
		"1.0":      false,
		"1.0.0-rc": false,
		"01.0.0":   false,
		"v1.0.0":   false,
	}
	for version, want := range tests {
		if got := semverPattern.MatchString(version); got != want {
			t.Fatalf("semverPattern.MatchString(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestPhase0AssetTypes(t *testing.T) {
	for _, assetType := range []string{"mcp", "skill", "agent", "workflow", "prompt_template"} {
		if _, ok := validTypes[assetType]; !ok {
			t.Fatalf("expected %q to be a valid Phase 0 asset type", assetType)
		}
	}
	for _, assetType := range []string{"prompt", "application", "repo_template", "eval_dataset", "healing_action"} {
		if _, ok := validTypes[assetType]; ok {
			t.Fatalf("did not expect legacy asset type %q to be valid", assetType)
		}
	}
}
