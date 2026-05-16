// Package prompts implements the prompt-template-service render API
// exposed by the ai-flow-authoring change. The LLM node in
// workflow-runtime calls POST /v1/render with {ref, variables} to get
// the rendered system/user/assistant_prefill strings + a conservative
// token estimate.
//
// The in-memory store ships with a small set of approved templates so
// the API contract is exercisable end-to-end; production wires a real
// store fed by the prompt asset registry.
package prompts

import (
	"context"
	"errors"
	"strings"
)

// RenderRequest is the body of POST /v1/render.
type RenderRequest struct {
	Ref       string         `json:"ref"`
	Variables map[string]any `json:"variables,omitempty"`
}

// RenderResponse is the rendered template.
type RenderResponse struct {
	System           string `json:"system,omitempty"`
	User             string `json:"user"`
	AssistantPrefill string `json:"assistant_prefill,omitempty"`
	TokenEstimate    int    `json:"token_estimate"`
}

// Renderer renders a prompt-template asset against the provided
// variables. Implementations enforce schema constraints declared on
// the template (required vars, types) — the stub here is permissive.
type Renderer interface {
	Render(ctx context.Context, req RenderRequest) (RenderResponse, error)
}

// ErrUnknownTemplate is returned when ref is not in the registry.
var ErrUnknownTemplate = errors.New("unknown_template")

// ErrBadRef is returned when ref does not parse as
// registry:prompt/<scope>/<name>@<semver>.
var ErrBadRef = errors.New("bad_template_ref")

// Template is the in-memory representation. Production fetches the
// equivalent from the prompt-registry / asset registry.
type Template struct {
	Ref              string
	System           string
	User             string // text with {{var}} placeholders
	AssistantPrefill string
}

// StubRenderer is a deterministic renderer with a small in-memory store.
type StubRenderer struct {
	Templates map[string]Template
}

// NewStubRenderer pre-loads a handful of templates that cover the
// reference flows shipped with the platform.
func NewStubRenderer() *StubRenderer {
	return &StubRenderer{
		Templates: map[string]Template{
			"registry:prompt/sdlc-product/email-classify@1.3.0": {
				Ref:    "registry:prompt/sdlc-product/email-classify@1.3.0",
				System: "You classify customer emails. Output JSON: {category, draft, confidence}.",
				User:   "Email body:\n{{body}}\n\nClassify into one of: urgent, billing, general.",
			},
			"registry:prompt/sdlc-product/refine-user-story@1.2.0": {
				Ref:    "registry:prompt/sdlc-product/refine-user-story@1.2.0",
				System: "You refine raw user stories into INVEST-compliant stories.",
				User:   "Story: {{story}}",
			},
		},
	}
}

// Render substitutes {{var}} placeholders with the variables map.
func (r *StubRenderer) Render(_ context.Context, req RenderRequest) (RenderResponse, error) {
	if !strings.HasPrefix(req.Ref, "registry:prompt/") {
		return RenderResponse{}, ErrBadRef
	}
	tpl, ok := r.Templates[req.Ref]
	if !ok {
		return RenderResponse{}, ErrUnknownTemplate
	}
	user := tpl.User
	for k, v := range req.Variables {
		user = strings.ReplaceAll(user, "{{"+k+"}}", asString(v))
	}
	full := tpl.System + "\n\n" + user
	return RenderResponse{
		System:           tpl.System,
		User:             user,
		AssistantPrefill: tpl.AssistantPrefill,
		// Conservative ~4 chars per token estimate, sufficient for cost previews.
		TokenEstimate: (len(full) / 4) + 1,
	}, nil
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return ""
	}
}
