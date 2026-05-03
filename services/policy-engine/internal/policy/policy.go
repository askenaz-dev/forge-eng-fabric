package policy

import (
	"fmt"
	"io"
	"sort"

	"github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"gopkg.in/yaml.v3"
)

type Decision string

const (
	Allow            Decision = "allow"
	RequiresApproval Decision = "requires_approval"
	Deny             Decision = "deny"
)

type EvaluateRequest struct {
	Principal          string         `json:"principal"`
	Action             string         `json:"action"`
	WorkspaceID        string         `json:"workspace_id"`
	OpenSpecID         *string        `json:"openspec_id,omitempty"`
	AssetID            *string        `json:"asset_id,omitempty"`
	Env                string         `json:"env"`
	Criticality        string         `json:"criticality"`
	TrustLevel         string         `json:"trust_level"`
	DataClassification string         `json:"data_classification"`
	Role               string         `json:"role"`
	Responsible        string         `json:"responsible"`
	Target             map[string]any `json:"target"`
}

type EvaluateResponse struct {
	Decision          Decision `json:"decision"`
	Rationale         string   `json:"rationale"`
	PolicyID          string   `json:"policy_id,omitempty"`
	RequiredApprovers []string `json:"required_approvers,omitempty"`
}

type Rule struct {
	ID                string   `yaml:"id" json:"id"`
	Priority          int      `yaml:"priority" json:"priority"`
	Decision          Decision `yaml:"decision" json:"decision"`
	Condition         string   `yaml:"condition" json:"condition"`
	Rationale         string   `yaml:"rationale" json:"rationale"`
	RequiredApprovers []string `yaml:"required_approvers" json:"required_approvers"`
}

type Document struct {
	Policies []Rule `yaml:"policies" json:"policies"`
}

type Engine struct {
    policies []Rule
}

func NewEngine(policies []Rule) *Engine {
	copyRules := append([]Rule(nil), policies...)
	sort.SliceStable(copyRules, func(i, j int) bool { return copyRules[i].Priority > copyRules[j].Priority })
	return &Engine{policies: copyRules}
}

func LoadYAML(r io.Reader) (*Engine, error) {
	var doc Document
	if err := yaml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, err
	}
	return NewEngine(doc.Policies), nil
}

func DefaultEngine() *Engine {
	return NewEngine([]Rule{
		{
			ID:                "deploy-prod-requires-approval",
			Priority:          100,
			Decision:          RequiresApproval,
			Condition:         `action == "deploy:prod" || env == "prod"`,
			Rationale:         "production-impacting actions require approval",
			RequiredApprovers: []string{"release-manager"},
		},
		{
			ID:        "default-allow",
			Priority:  1,
			Decision:  Allow,
			Condition: "true",
			Rationale: "no restrictive policy matched",
		},
	})
}

func (e *Engine) Evaluate(req EvaluateRequest) (EvaluateResponse, error) {
    matched := []Rule{}
    for _, rule := range e.policies {
        ok, err := evalCEL(rule.Condition, req)
        if err != nil {
            return EvaluateResponse{}, fmt.Errorf("policy %s: %w", rule.ID, err)
        }
        if ok {
            matched = append(matched, rule)
        }
    }
    if len(matched) == 0 {
        return EvaluateResponse{Decision: Allow, Rationale: "no policy matched"}, nil
    }
    return mostRestrictive(matched), nil
}

// EvaluatePipelineGate is a convenience to evaluate pipeline gate events.
func (e *Engine) EvaluatePipelineGate(event map[string]any) (EvaluateResponse, error) {
    req := EvaluateRequest{
        Principal:          strOr(event["principal"]),
        Action:             strOr(event["action"]),
        WorkspaceID:        strOr(event["workspace_id"]),
        Env:                strOr(event["env"]),
        Criticality:        strOr(event["criticality"]),
        TrustLevel:         strOr(event["trust_level"]),
        DataClassification: strOr(event["data_classification"]),
        Role:               strOr(event["role"]),
        Responsible:        strOr(event["responsible"]),
        Target:             mapOr(event["target"]),
    }
    return e.Evaluate(req)
}

func strOr(v any) string {
    if s, ok := v.(string); ok {
        return s
    }
    return ""
}

func mapOr(v any) map[string]any {
    if m, ok := v.(map[string]any); ok {
        return m
    }
    return map[string]any{}
}

func mostRestrictive(rules []Rule) EvaluateResponse {
	for _, decision := range []Decision{Deny, RequiresApproval, Allow} {
		for _, rule := range rules {
			if rule.Decision == decision {
				return EvaluateResponse{
					Decision:          rule.Decision,
					Rationale:         rule.Rationale,
					PolicyID:          rule.ID,
					RequiredApprovers: rule.RequiredApprovers,
				}
			}
		}
	}
	return EvaluateResponse{Decision: Allow, Rationale: "no restrictive policy matched"}
}

func evalCEL(condition string, req EvaluateRequest) (bool, error) {
	if condition == "" {
		return true, nil
	}
	env, err := cel.NewEnv(
		cel.Variable("principal", cel.StringType),
		cel.Variable("action", cel.StringType),
		cel.Variable("workspace_id", cel.StringType),
		cel.Variable("openspec_id", cel.StringType),
		cel.Variable("asset_id", cel.StringType),
		cel.Variable("env", cel.StringType),
		cel.Variable("criticality", cel.StringType),
		cel.Variable("trust_level", cel.StringType),
		cel.Variable("data_classification", cel.StringType),
		cel.Variable("role", cel.StringType),
		cel.Variable("responsible", cel.StringType),
		cel.Variable("target", cel.MapType(cel.StringType, cel.DynType)),
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
		"principal":           req.Principal,
		"action":              req.Action,
		"workspace_id":        req.WorkspaceID,
		"openspec_id":         strPtr(req.OpenSpecID),
		"asset_id":            strPtr(req.AssetID),
		"env":                 req.Env,
		"criticality":         req.Criticality,
		"trust_level":         req.TrustLevel,
		"data_classification": req.DataClassification,
		"role":                req.Role,
		"responsible":         req.Responsible,
		"target":              req.Target,
	})
	if err != nil {
		return false, err
	}
	return out == celtypes.True, nil
}

func strPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
