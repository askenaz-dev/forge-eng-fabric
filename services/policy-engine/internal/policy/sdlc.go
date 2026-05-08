package policy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"gopkg.in/yaml.v3"
)

type SDLCGateTemplate struct {
	ID                string                    `yaml:"id" json:"id"`
	Type              string                    `yaml:"type" json:"type"`
	Phase             string                    `yaml:"phase" json:"phase"`
	Gate              string                    `yaml:"gate" json:"gate"`
	MinCriticality    string                    `yaml:"min_criticality" json:"min_criticality,omitempty"`
	Condition         string                    `yaml:"condition" json:"condition"`
	Rationale         string                    `yaml:"rationale" json:"rationale"`
	Thresholds        map[string]map[string]any `yaml:"thresholds" json:"thresholds,omitempty"`
	RequiredApprovers []string                  `yaml:"required_approvers" json:"required_approvers,omitempty"`
}

type SDLCGateDocument struct {
	Gates []SDLCGateTemplate `yaml:"sdlc_gates" json:"sdlc_gates"`
}

type SDLCGateEvaluateRequest struct {
	WorkspaceID  string         `json:"workspace_id"`
	InitiativeID string         `json:"initiative_id"`
	Phase        string         `json:"phase"`
	Gate         string         `json:"gate,omitempty"`
	Criticality  string         `json:"criticality"`
	Evidence     map[string]any `json:"evidence"`
	Thresholds   map[string]any `json:"thresholds,omitempty"`
}

type SDLCGateEvaluateResponse struct {
	Results []SDLCGateResult `json:"results"`
}

type SDLCGateResult struct {
	PolicyID   string         `json:"policy_id"`
	Phase      string         `json:"phase"`
	Gate       string         `json:"gate"`
	Outcome    string         `json:"outcome"`
	Reason     string         `json:"reason,omitempty"`
	Thresholds map[string]any `json:"thresholds,omitempty"`
}

type SDLCGateEngine struct {
	templates []SDLCGateTemplate
}

func LoadSDLCGateTemplates(data []byte) (*SDLCGateEngine, error) {
	var doc SDLCGateDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return NewSDLCGateEngine(doc.Gates), nil
}

func NewSDLCGateEngine(templates []SDLCGateTemplate) *SDLCGateEngine {
	copyTemplates := append([]SDLCGateTemplate(nil), templates...)
	sort.SliceStable(copyTemplates, func(i, j int) bool { return copyTemplates[i].ID < copyTemplates[j].ID })
	return &SDLCGateEngine{templates: copyTemplates}
}

func (e *SDLCGateEngine) Templates() []SDLCGateTemplate {
	return append([]SDLCGateTemplate(nil), e.templates...)
}

func (e *SDLCGateEngine) EvaluateSDLCGate(req SDLCGateEvaluateRequest) (SDLCGateEvaluateResponse, error) {
	criticality := strings.ToLower(req.Criticality)
	if criticality == "" {
		criticality = "medium"
	}
	results := []SDLCGateResult{}
	for _, template := range e.templates {
		if template.Type != "sdlc-gate" {
			continue
		}
		if template.Phase != req.Phase {
			continue
		}
		if req.Gate != "" && template.Gate != req.Gate {
			continue
		}
		if template.MinCriticality != "" && !criticalityAtLeast(criticality, template.MinCriticality) {
			results = append(results, SDLCGateResult{
				PolicyID: template.ID,
				Phase:    template.Phase,
				Gate:     template.Gate,
				Outcome:  "skipped",
				Reason:   "below_min_criticality",
			})
			continue
		}
		thresholds := mergeThresholds(template.Thresholds[criticality], req.Thresholds)
		passed, err := evalSDLCGateCEL(template.Condition, req, thresholds)
		if err != nil {
			return SDLCGateEvaluateResponse{}, fmt.Errorf("sdlc gate %s: %w", template.ID, err)
		}
		result := SDLCGateResult{
			PolicyID:   template.ID,
			Phase:      template.Phase,
			Gate:       template.Gate,
			Outcome:    "passed",
			Thresholds: thresholds,
		}
		if !passed {
			result.Outcome = "failed"
			result.Reason = template.Rationale
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return SDLCGateEvaluateResponse{}, fmt.Errorf("no_sdlc_gate_templates_matched")
	}
	return SDLCGateEvaluateResponse{Results: results}, nil
}

func mergeThresholds(defaults, overrides map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range defaults {
		out[k] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out
}

func evalSDLCGateCEL(condition string, req SDLCGateEvaluateRequest, thresholds map[string]any) (bool, error) {
	if condition == "" {
		return true, nil
	}
	env, err := cel.NewEnv(
		cel.Variable("workspace_id", cel.StringType),
		cel.Variable("initiative_id", cel.StringType),
		cel.Variable("phase", cel.StringType),
		cel.Variable("gate", cel.StringType),
		cel.Variable("criticality", cel.StringType),
		cel.Variable("evidence", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("thresholds", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return false, err
	}
	ast, issues := env.Compile(condition)
	if issues != nil && issues.Err() != nil {
		return false, issues.Err()
	}
	program, err := env.Program(ast)
	if err != nil {
		return false, err
	}
	out, _, err := program.Eval(map[string]any{
		"workspace_id":  req.WorkspaceID,
		"initiative_id": req.InitiativeID,
		"phase":         req.Phase,
		"gate":          req.Gate,
		"criticality":   req.Criticality,
		"evidence":      req.Evidence,
		"thresholds":    thresholds,
	})
	if err != nil {
		return false, err
	}
	return out == celtypes.True, nil
}

func criticalityAtLeast(value, minimum string) bool {
	rank := map[string]int{"low": 1, "medium": 2, "high": 3, "critical": 4}
	v := rank[strings.ToLower(value)]
	if v == 0 {
		v = rank["medium"]
	}
	return v >= rank[strings.ToLower(minimum)]
}
