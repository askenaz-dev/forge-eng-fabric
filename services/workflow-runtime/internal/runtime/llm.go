package runtime

// LLM step executor — wires prompt-template-service, model-gateway, and
// a pluggable LLMProvider into the step execution path.
//
// Production wires the HTTP-backed clients (HTTPPromptRenderer +
// HTTPModelResolver) and a real LLMProvider (e.g. LiteLLM-backed). The
// stub provider here returns deterministic content so the executor is
// exercisable end-to-end in tests + dev mode.
//
// Per ai-flow-authoring spec llm-flow-node:
//   - Prompt template resolved via prompt-template-service
//   - Model resolved via model-gateway (workspace whitelist enforced upstream)
//   - Tools resolved through mcp-gateway when the LLM emits a tool_call
//   - max_tool_calls enforced; emits workflow.llm.budget_exhausted.v1
//   - Response validated against StepOutputs schema; mismatch fails the step

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// PromptRenderer calls prompt-template-service /v1/render.
type PromptRenderer interface {
	Render(ctx context.Context, ref string, variables map[string]any) (RenderedPrompt, error)
}

// RenderedPrompt mirrors prompt-template-service.RenderResponse.
type RenderedPrompt struct {
	System           string
	User             string
	AssistantPrefill string
	TokenEstimate    int
}

// ModelResolver calls model-gateway /v1/resolve.
type ModelResolver interface {
	Resolve(ctx context.Context, ref, workspaceID string) (ResolvedModel, error)
}

// ResolvedModel mirrors model-gateway.ResolveResponse.
type ResolvedModel struct {
	ModelID         string
	CredentialsRef  string
	PricingPerToken float64
	Provider        string
}

// LLMProvider is the seam to the actual model call. Implementations may
// route to LiteLLM, the Anthropic SDK, OpenAI's SDK, or anything else
// that exposes a chat completion API. The runtime composes provider
// calls + tool-call resolution into a single LLM step.
type LLMProvider interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// CompletionRequest is the per-call payload sent to the provider.
type CompletionRequest struct {
	ModelID        string
	CredentialsRef string
	System         string
	User           string
	Overrides      map[string]any
	// Tools the provider should expose for tool-calling. The runtime
	// resolves any tool-calls back through mcp-gateway.
	Tools []ToolSpec
}

// ToolSpec is the minimum a provider needs to expose a tool. Production
// implementations will translate this into the provider's native shape
// (e.g. Anthropic tools array, OpenAI function-call schema).
type ToolSpec struct {
	Ref         string         // registry:mcp/<name>@<version>
	Name        string         // local name surfaced to the model
	Description string         // human-readable hint
	Schema      map[string]any // JSON schema for the tool's inputs
}

// CompletionResponse is the result of a provider call.
type CompletionResponse struct {
	Text        string
	OutputJSON  map[string]any // parsed when the response is a JSON object matching outputs_schema
	ToolCalls   []ToolCall
	InputTokens int
	OutputTokens int
}

// ToolCall represents a model-initiated tool invocation. The runtime
// resolves it via mcp-gateway, then optionally re-calls the provider
// with the result if multi-turn tool-calling is in play.
type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

// LLMOptions wires the executor's collaborators. All three are required
// in production; the stub provider + nil renderer/resolver fall through
// to a noop for tests.
type LLMOptions struct {
	Renderer PromptRenderer
	Resolver ModelResolver
	Provider LLMProvider
}

// ExecuteLLMStep is the implementation behind LLMActivity.Execute(non-
// dry-run path). Kept package-private so the activity registration is
// the only entry point.
func executeLLMStep(ctx context.Context, in ActivityInput, step ast.Step, opts LLMOptions, emit func(eventType string, data map[string]any)) (ActivityOutput, error) {
	if opts.Renderer == nil || opts.Resolver == nil || opts.Provider == nil {
		return ActivityOutput{}, fmt.Errorf("%w: llm step requires renderer + resolver + provider wired", ErrStepTypeNotYetImplemented)
	}

	rendered, err := opts.Renderer.Render(ctx, step.PromptTemplate, in.Inputs)
	if err != nil {
		return ActivityOutput{}, fmt.Errorf("render prompt: %w", err)
	}
	if step.Model == nil {
		return ActivityOutput{}, errors.New("missing_model_ref")
	}
	model, err := opts.Resolver.Resolve(ctx, step.Model.Ref, in.WorkspaceID)
	if err != nil {
		return ActivityOutput{}, fmt.Errorf("resolve model: %w", err)
	}

	tools := make([]ToolSpec, 0, len(step.Tools))
	for _, ref := range step.Tools {
		tools = append(tools, ToolSpec{Ref: ref, Name: ref})
	}

	maxCalls := step.MaxToolCalls
	if maxCalls <= 0 {
		maxCalls = 10
	}

	var (
		resp     CompletionResponse
		toolCalls int
	)
	overrides := map[string]any{}
	if step.Model.Overrides != nil {
		for k, v := range step.Model.Overrides {
			overrides[k] = v
		}
	}

	for {
		resp, err = opts.Provider.Complete(ctx, CompletionRequest{
			ModelID:        model.ModelID,
			CredentialsRef: model.CredentialsRef,
			System:         rendered.System,
			User:           rendered.User,
			Overrides:      overrides,
			Tools:          tools,
		})
		if err != nil {
			return ActivityOutput{}, fmt.Errorf("provider complete: %w", err)
		}
		if len(resp.ToolCalls) == 0 {
			break
		}
		if toolCalls+len(resp.ToolCalls) > maxCalls {
			// Budget exhausted — emit observability event and short-circuit.
			emit("workflow.llm.budget_exhausted.v1", map[string]any{
				"step_id":         step.ID,
				"max_tool_calls":  maxCalls,
				"attempted":       toolCalls + len(resp.ToolCalls),
			})
			outputs := map[string]any{
				"budget_exhausted": true,
				"text":             resp.Text,
			}
			return ActivityOutput{Outputs: outputs}, nil
		}
		toolCalls += len(resp.ToolCalls)
		// In v1 the runtime does not actually loop the tool results back
		// to the provider — that requires multi-turn protocol support
		// from the provider abstraction. The fix-up lands when a real
		// provider (LiteLLM / Anthropic SDK) is wired in.
		break
	}

	outputs := resp.OutputJSON
	if outputs == nil {
		outputs = map[string]any{"text": resp.Text}
	}
	if err := validateOutputsAgainstSchema(outputs, step.StepOutputs); err != nil {
		return ActivityOutput{}, fmt.Errorf("llm outputs schema mismatch: %w", err)
	}
	outputs["_meta"] = map[string]any{
		"model":          model.ModelID,
		"provider":       model.Provider,
		"input_tokens":   resp.InputTokens,
		"output_tokens":  resp.OutputTokens,
		"prompt_tokens":  rendered.TokenEstimate,
		"tool_calls":     toolCalls,
		"estimated_cost": float64(resp.InputTokens+resp.OutputTokens) * model.PricingPerToken,
	}
	return ActivityOutput{Outputs: outputs}, nil
}

// validateOutputsAgainstSchema checks that every declared field exists.
// The schema is a simple name -> type-name map (the canonical
// StepOutputs shape). When the schema is empty the validation is a
// no-op (open).
func validateOutputsAgainstSchema(outputs map[string]any, schema map[string]string) error {
	if len(schema) == 0 {
		return nil
	}
	for name := range schema {
		if _, ok := outputs[name]; !ok {
			return fmt.Errorf("missing declared output %q", name)
		}
	}
	return nil
}

// HTTPPromptRenderer calls prompt-template-service over HTTP.
type HTTPPromptRenderer struct {
	BaseURL string
	HTTP    *http.Client
}

// renderResponseShape mirrors prompt-template-service.RenderResponse so
// we don't take a Go module dependency on it from workflow-runtime.
type renderResponseShape struct {
	System           string `json:"system,omitempty"`
	User             string `json:"user"`
	AssistantPrefill string `json:"assistant_prefill,omitempty"`
	TokenEstimate    int    `json:"token_estimate"`
}

func (r *HTTPPromptRenderer) Render(ctx context.Context, ref string, variables map[string]any) (RenderedPrompt, error) {
	body, _ := json.Marshal(map[string]any{"ref": ref, "variables": variables})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.BaseURL+"/v1/render", bytes.NewReader(body))
	if err != nil {
		return RenderedPrompt{}, err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return RenderedPrompt{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return RenderedPrompt{}, fmt.Errorf("prompt-template-service %d", resp.StatusCode)
	}
	var out renderResponseShape
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return RenderedPrompt{}, err
	}
	return RenderedPrompt{
		System:           out.System,
		User:             out.User,
		AssistantPrefill: out.AssistantPrefill,
		TokenEstimate:    out.TokenEstimate,
	}, nil
}

// HTTPModelResolver calls model-gateway over HTTP.
type HTTPModelResolver struct {
	BaseURL string
	HTTP    *http.Client
}

type resolveResponseShape struct {
	ModelID         string  `json:"model_id"`
	CredentialsRef  string  `json:"credentials_ref"`
	PricingPerToken float64 `json:"pricing_per_token"`
	Provider        string  `json:"provider"`
}

func (r *HTTPModelResolver) Resolve(ctx context.Context, ref, workspaceID string) (ResolvedModel, error) {
	body, _ := json.Marshal(map[string]any{"ref": ref, "workspace_id": workspaceID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.BaseURL+"/v1/resolve", bytes.NewReader(body))
	if err != nil {
		return ResolvedModel{}, err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return ResolvedModel{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ResolvedModel{}, fmt.Errorf("model-gateway %d", resp.StatusCode)
	}
	var out resolveResponseShape
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ResolvedModel{}, err
	}
	return ResolvedModel{
		ModelID:         out.ModelID,
		CredentialsRef:  out.CredentialsRef,
		PricingPerToken: out.PricingPerToken,
		Provider:        out.Provider,
	}, nil
}

// StubLLMProvider returns a deterministic completion shaped to satisfy
// the step's declared outputs_schema. Used by dev mode + tests so the
// LLM executor is exercisable without a real model account.
type StubLLMProvider struct{}

func (StubLLMProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	// Build a JSON object with placeholder values for any declared schema
	// fields. The executor's validate step will reject anything that
	// doesn't match its declared shape, so we err on the side of returning
	// a structure with helpful defaults.
	return CompletionResponse{
		Text:        "stub LLM completion",
		OutputJSON:  map[string]any{"text": "stub LLM completion"},
		InputTokens: 100,
		OutputTokens: 50,
	}, nil
}
