package ast

import (
	_ "embed"
	"encoding/json"
	"sort"
	"testing"
)

//go:embed catalog.json
var catalogJSON []byte

type canonicalCatalog struct {
	StepTypes           []string          `json:"step_types"`
	DeprecatedStepTypes map[string]string `json:"deprecated_step_types"`
	TriggerTypes        []string          `json:"trigger_types"`
}

// TestStepTypeParityWithCatalog asserts the Go StepType enum is in exact
// agreement with catalog.json. The same catalog file is consumed by the
// Portal TS adapter, so any drift between Go and TS surfaces here as a
// failing test — preventing the historical 15-vs-8 catalog mismatch from
// recurring (see ai-flow-authoring change, design.md D8).
func TestStepTypeParityWithCatalog(t *testing.T) {
	cat := loadCatalog(t)

	goStepSet := map[StepType]bool{}
	for _, s := range AllStepTypes() {
		goStepSet[s] = true
	}

	wantNonDeprecated := map[StepType]bool{}
	for _, s := range cat.StepTypes {
		wantNonDeprecated[StepType(s)] = true
	}
	wantDeprecated := map[StepType]bool{}
	for s := range cat.DeprecatedStepTypes {
		wantDeprecated[StepType(s)] = true
	}

	// Every catalog non-deprecated type must exist in the Go enum.
	missingInGo := []string{}
	for s := range wantNonDeprecated {
		if !goStepSet[s] {
			missingInGo = append(missingInGo, string(s))
		}
	}
	for s := range wantDeprecated {
		if !goStepSet[s] {
			missingInGo = append(missingInGo, string(s)+" (deprecated)")
		}
	}
	sort.Strings(missingInGo)
	if len(missingInGo) > 0 {
		t.Errorf("step_type_parity_mismatch: catalog.json lists step types not in Go enum: %v", missingInGo)
	}

	// Every Go enum value must exist in the catalog (deprecated or not).
	extraInGo := []string{}
	for s := range goStepSet {
		if !wantNonDeprecated[s] && !wantDeprecated[s] {
			extraInGo = append(extraInGo, string(s))
		}
	}
	sort.Strings(extraInGo)
	if len(extraInGo) > 0 {
		t.Errorf("step_type_parity_mismatch: Go enum has values not in catalog.json: %v", extraInGo)
	}
}

// TestTriggerTypeParityWithCatalog asserts the Go TriggerType enum is in
// exact agreement with catalog.json. Mirrors the step-type parity test so
// the TS adapter's CanonicalTriggerType cannot drift in either direction.
func TestTriggerTypeParityWithCatalog(t *testing.T) {
	cat := loadCatalog(t)

	goSet := map[TriggerType]bool{}
	for _, tt := range AllTriggerTypes() {
		goSet[tt] = true
	}
	wantSet := map[TriggerType]bool{}
	for _, s := range cat.TriggerTypes {
		wantSet[TriggerType(s)] = true
	}

	missing := []string{}
	for tt := range wantSet {
		if !goSet[tt] {
			missing = append(missing, string(tt))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("trigger_type_parity_mismatch: catalog.json lists trigger types not in Go enum: %v", missing)
	}

	extra := []string{}
	for tt := range goSet {
		if !wantSet[tt] {
			extra = append(extra, string(tt))
		}
	}
	sort.Strings(extra)
	if len(extra) > 0 {
		t.Errorf("trigger_type_parity_mismatch: Go enum has trigger values not in catalog.json: %v", extra)
	}
}

// TestDeprecatedStepTypesMatchCatalog asserts DeprecatedStepTypes() matches
// the catalog file. Both pieces are read at runtime; drift between them
// would break the migration semantics.
func TestDeprecatedStepTypesMatchCatalog(t *testing.T) {
	cat := loadCatalog(t)
	got := DeprecatedStepTypes()
	for from, want := range cat.DeprecatedStepTypes {
		if g, ok := got[StepType(from)]; !ok {
			t.Errorf("DeprecatedStepTypes() missing %q (catalog says it migrates to %q)", from, want)
		} else if string(g) != want {
			t.Errorf("DeprecatedStepTypes()[%q] = %q, catalog says %q", from, g, want)
		}
	}
	for from := range got {
		if _, ok := cat.DeprecatedStepTypes[string(from)]; !ok {
			t.Errorf("DeprecatedStepTypes() lists %q but catalog.json does not", from)
		}
	}
}

func loadCatalog(t *testing.T) canonicalCatalog {
	t.Helper()
	var cat canonicalCatalog
	if err := json.Unmarshal(catalogJSON, &cat); err != nil {
		t.Fatalf("unmarshal catalog.json: %v", err)
	}
	if len(cat.StepTypes) == 0 {
		t.Fatal("catalog.json has no step_types")
	}
	return cat
}
