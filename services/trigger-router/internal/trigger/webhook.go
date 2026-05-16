package trigger

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/forge-eng-fabric/pkg/workflow/ast"
)

// SecretResolver fetches a webhook signing secret by reference. The
// trigger's config carries something like `{secret_ref: ws:secret:gh-hook}`;
// production wires the platform secrets broker, tests use StaticSecrets.
type SecretResolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

// StaticSecrets is an in-memory secret resolver for tests / dev mode.
type StaticSecrets map[string]string

func (s StaticSecrets) Resolve(_ context.Context, ref string) (string, error) {
	v, ok := s[ref]
	if !ok {
		return "", errors.New("secret_not_found")
	}
	return v, nil
}

// WebhookHandler serves the /v1/hooks/in/{workflow_id}/{trigger_id}
// endpoint. Verifies HMAC-SHA256 signatures using a per-trigger secret,
// then dispatches the request body as the trigger payload.
type WebhookHandler struct {
	Registry   *Registry
	Dispatcher *Dispatcher
	Secrets    SecretResolver
}

// ServeHTTP routes POST /v1/hooks/in/{workflow_id}/{trigger_id}.
// Path parsing is manual so this handler can be mounted with any router
// (or with the std-lib mux at /v1/hooks/in/).
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	const prefix = "/v1/hooks/in/"
	path := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "bad_path: expected /v1/hooks/in/{workflow_id}/{trigger_id}", http.StatusBadRequest)
		return
	}
	workflowID, triggerID := parts[0], parts[1]

	sub, ok := h.Registry.Lookup(workflowID, triggerID, "")
	if !ok {
		http.Error(w, "trigger_not_registered", http.StatusNotFound)
		return
	}
	if sub.Type != ast.TriggerWebhookIn {
		http.Error(w, "trigger_type_mismatch", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read_body", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	if secretRef, _ := sub.Config["secret_ref"].(string); secretRef != "" {
		secret, err := h.Secrets.Resolve(r.Context(), secretRef)
		if err != nil {
			http.Error(w, "secret_resolve_failed", http.StatusInternalServerError)
			return
		}
		sigHeader := r.Header.Get("X-Forge-Signature")
		if !verifyHMAC(secret, body, sigHeader) {
			http.Error(w, "invalid_signature", http.StatusUnauthorized)
			return
		}
	}

	payload := map[string]any{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			payload = map[string]any{"raw": string(body)}
		}
	}
	execID, err := h.Dispatcher.Fire(r.Context(), sub, payload)
	if err != nil {
		if errors.Is(err, ErrDropConcurrency) {
			http.Error(w, "drop_concurrency", http.StatusConflict)
			return
		}
		http.Error(w, "dispatch_failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"execution_id": execID})
}

// verifyHMAC validates an X-Forge-Signature header of the form
// "sha256=<hex>" against the request body. Constant-time comparison.
func verifyHMAC(secret string, body []byte, sigHeader string) bool {
	if !strings.HasPrefix(sigHeader, "sha256=") {
		return false
	}
	got, err := hex.DecodeString(strings.TrimPrefix(sigHeader, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := mac.Sum(nil)
	return hmac.Equal(got, want)
}
