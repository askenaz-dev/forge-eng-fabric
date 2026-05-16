// Package gateway implements the model-gateway resolution API exposed
// by the ai-flow-authoring change. The LLM node in workflow-runtime
// calls POST /v1/resolve with {ref, workspace_id} and receives the
// concrete model id, credentials reference, and pricing it needs to
// dispatch the model call.
//
// This is the stable API contract; the actual model routing (LiteLLM
// integration, multi-provider selection, fall-through, etc.) is the
// subject of follow-up work. The in-memory implementation here returns
// deterministic mock data so the workflow-runtime LLM executor can
// integrate against a real HTTP surface during the rollout window.
package gateway

import (
	"context"
	"errors"
	"strings"
)

// ResolveRequest is the body of POST /v1/resolve.
type ResolveRequest struct {
	Ref         string `json:"ref"`
	WorkspaceID string `json:"workspace_id"`
}

// ResolveResponse is the resolution result.
type ResolveResponse struct {
	ModelID         string  `json:"model_id"`
	CredentialsRef  string  `json:"credentials_ref"`
	PricingPerToken float64 `json:"pricing_per_token"`
	Provider        string  `json:"provider"`
}

// Resolver returns model resolution for an LLM node. Production wires a
// LiteLLM-backed implementation; this package ships a stub registry.
type Resolver interface {
	Resolve(ctx context.Context, req ResolveRequest) (ResolveResponse, error)
}

// Whitelist allows a workspace to restrict the set of models a flow can
// resolve. Empty whitelist = no restriction.
type Whitelist interface {
	AllowedModels(ctx context.Context, workspaceID string) ([]string, error)
}

// ErrModelNotWhitelisted is returned when the resolved model is not in
// the workspace's allowed_models list. Matches the lint rule of the
// same name in pkg/workflow/lint.
var ErrModelNotWhitelisted = errors.New("model_not_whitelisted")

// ErrBadRef is returned when ref does not parse as
// `gateway:model/<model-id>@<channel>`.
var ErrBadRef = errors.New("bad_model_ref")

// ErrUnknownModel is returned when the model id is unknown to the
// stub registry. Production resolvers may be more permissive.
var ErrUnknownModel = errors.New("unknown_model")

// StubResolver is a deterministic resolver used in dev mode and tests.
// Known models map to stable pricing/credentials values.
type StubResolver struct {
	WS Whitelist
}

// KnownModels maps a model id to its pricing in USD per token and the
// upstream provider. Extend as needed; production fetches this from a
// pricing service.
var KnownModels = map[string]struct {
	Provider        string
	PricingPerToken float64
}{
	"claude-opus-4-7":           {Provider: "anthropic", PricingPerToken: 0.000015},
	"claude-sonnet-4-6":         {Provider: "anthropic", PricingPerToken: 0.000003},
	"claude-haiku-4-5-20251001": {Provider: "anthropic", PricingPerToken: 0.0000008},
	"gpt-4o":                    {Provider: "openai", PricingPerToken: 0.000005},
	"gpt-4o-mini":               {Provider: "openai", PricingPerToken: 0.00000015},
}

// Resolve parses ref, looks up the model, enforces the workspace
// whitelist, and returns the resolution.
func (r *StubResolver) Resolve(ctx context.Context, req ResolveRequest) (ResolveResponse, error) {
	modelID, _, err := parseRef(req.Ref)
	if err != nil {
		return ResolveResponse{}, err
	}
	meta, ok := KnownModels[modelID]
	if !ok {
		return ResolveResponse{}, ErrUnknownModel
	}
	if r.WS != nil {
		allowed, err := r.WS.AllowedModels(ctx, req.WorkspaceID)
		if err == nil && !inList(allowed, modelID) {
			return ResolveResponse{}, ErrModelNotWhitelisted
		}
	}
	return ResolveResponse{
		ModelID:         modelID,
		CredentialsRef:  "ws:credentials:" + req.WorkspaceID + ":" + meta.Provider,
		PricingPerToken: meta.PricingPerToken,
		Provider:        meta.Provider,
	}, nil
}

// parseRef parses gateway:model/<model-id>@<channel> into (model_id, channel).
func parseRef(ref string) (string, string, error) {
	if !strings.HasPrefix(ref, "gateway:model/") {
		return "", "", ErrBadRef
	}
	body := strings.TrimPrefix(ref, "gateway:model/")
	at := strings.LastIndex(body, "@")
	if at <= 0 || at == len(body)-1 {
		return "", "", ErrBadRef
	}
	return body[:at], body[at+1:], nil
}

func inList(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// StaticWhitelist is a Whitelist for tests / dev mode.
type StaticWhitelist map[string][]string

func (s StaticWhitelist) AllowedModels(_ context.Context, workspaceID string) ([]string, error) {
	return s[workspaceID], nil
}
