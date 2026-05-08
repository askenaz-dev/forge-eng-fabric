package policy

import "testing"

const sampleSDLCGateYAML = `
sdlc_gates:
  - id: require-test-coverage
    type: sdlc-gate
    phase: development
    gate: coverage
    condition: 'evidence["coverage_percent"] >= thresholds["coverage_percent"]'
    rationale: coverage is below criticality threshold
    thresholds:
      low: {coverage_percent: 70}
      medium: {coverage_percent: 75}
      high: {coverage_percent: 80}
      critical: {coverage_percent: 85}
  - id: require-threat-model
    type: sdlc-gate
    phase: design
    gate: threat_model_present
    min_criticality: medium
    condition: 'evidence["threat_model_present"] == true'
    rationale: threat model is required for medium or higher criticality
`

func TestSDLCGateCoverageUsesCriticalityThreshold(t *testing.T) {
	engine, err := LoadSDLCGateTemplates([]byte(sampleSDLCGateYAML))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	resp, err := engine.EvaluateSDLCGate(SDLCGateEvaluateRequest{
		Phase:       "development",
		Gate:        "coverage",
		Criticality: "high",
		Evidence:    map[string]any{"coverage_percent": 78},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Outcome != "failed" {
		t.Fatalf("expected failed coverage gate, got %+v", resp.Results)
	}
	if resp.Results[0].Thresholds["coverage_percent"] != 80 {
		t.Fatalf("expected high threshold 80, got %+v", resp.Results[0].Thresholds)
	}

	resp, err = engine.EvaluateSDLCGate(SDLCGateEvaluateRequest{
		Phase:       "development",
		Gate:        "coverage",
		Criticality: "high",
		Evidence:    map[string]any{"coverage_percent": 82},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if resp.Results[0].Outcome != "passed" {
		t.Fatalf("expected passed coverage gate, got %+v", resp.Results[0])
	}
}

func TestSDLCGateSkipsBelowMinCriticality(t *testing.T) {
	engine, err := LoadSDLCGateTemplates([]byte(sampleSDLCGateYAML))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	resp, err := engine.EvaluateSDLCGate(SDLCGateEvaluateRequest{
		Phase:       "design",
		Gate:        "threat_model_present",
		Criticality: "low",
		Evidence:    map[string]any{},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if resp.Results[0].Outcome != "skipped" {
		t.Fatalf("expected skipped gate, got %+v", resp.Results[0])
	}
}
