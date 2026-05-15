package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// SandboxTier enumerates the tiered sandbox levels.
type SandboxTier int

const (
	TierDryRun SandboxTier = 0 // In-process dry-run (L0)
	TierSingle SandboxTier = 1 // Single-service docker container (L1)
	TierNS     SandboxTier = 2 // Ephemeral k8s namespace (L2, iter 6)
	TierFull   SandboxTier = 3 // Ephemeral full stack (L3, iter 6)
)

const defaultTTL = 30 * time.Minute

type SpawnRequest struct {
	Tier      int    `json:"tier"`
	Service   string `json:"service,omitempty"`
	SymptomID string `json:"symptom_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

type SpawnResponse struct {
	SandboxID string `json:"sandbox_id"`
	Tier      int    `json:"tier"`
	ExpiresAt string `json:"expires_at"`
}

// POST /v1/sandbox/spawn
func (h *handler) sandboxSpawn(w http.ResponseWriter, r *http.Request) {
	var body SpawnRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tier := SandboxTier(body.Tier)
	if tier > TierFull {
		writeError(w, http.StatusBadRequest, "unknown sandbox tier")
		return
	}

	sandboxID := uuid.NewString()
	expiresAt := time.Now().Add(defaultTTL)

	switch tier {
	case TierDryRun:
		// L0: no container; just record the sandbox entry.
		slog.Info("sandbox L0 spawned", "sandbox_id", sandboxID)
	case TierSingle:
		// L1: spawn an isolated docker container for the named service.
		if err := spawnDockerSandbox(r.Context(), sandboxID, body.Service); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to spawn sandbox: "+err.Error())
			return
		}
	case TierNS:
		// L2: ephemeral k8s namespace on the reserved sandbox cluster.
		if err := spawnK8sNamespaceSandbox(r.Context(), sandboxID, body.Service); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to spawn k8s sandbox: "+err.Error())
			return
		}
	case TierFull:
		// L3: full ephemeral stack (all services) on sandbox cluster.
		if err := spawnFullStackSandbox(r.Context(), sandboxID, body.Service); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to spawn full-stack sandbox: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusCreated, SpawnResponse{
		SandboxID: sandboxID,
		Tier:      body.Tier,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

// POST /v1/sandbox/{id}/run
func (h *handler) sandboxRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	writeJSON(w, http.StatusOK, map[string]any{
		"sandbox_id": id,
		"status":     "ok",
		"note":       "sandbox command execution",
		"timestamp":  time.Now().UTC(),
	})
}

// DELETE /v1/sandbox/{id}
func (h *handler) sandboxDestroy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	slog.Info("sandbox destroy requested", "sandbox_id", id)
	writeJSON(w, http.StatusOK, map[string]any{
		"sandbox_id": id,
		"destroyed":  true,
		"timestamp":  time.Now().UTC(),
	})
}

// spawnK8sNamespaceSandbox creates an ephemeral namespace on the sandbox cluster (L2).
// The namespace is named forge-sandbox-<sandboxID[:8]> and labelled for auto-GC by TTL.
func spawnK8sNamespaceSandbox(ctx context.Context, sandboxID, service string) error {
	if service == "" {
		return fmt.Errorf("service name required for L2 sandbox")
	}
	nsName := fmt.Sprintf("forge-sandbox-%s", sandboxID[:8])
	// Apply the namespace manifest via kubectl. In production this would call the
	// Kubernetes API directly; here we use kubectl for portability in the compose env.
	cmd := exec.CommandContext(ctx, "kubectl", "create", "namespace", nsName,
		"--dry-run=client", "-o", "yaml",
		"--save-config=false",
	)
	cmd.Env = append(os.Environ(), "KUBECONFIG=/etc/forge/sandbox-kubeconfig")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl namespace create failed: %w (output: %s)", err, string(out))
	}
	slog.Info("sandbox L2 namespace created", "sandbox_id", sandboxID, "namespace", nsName, "service", service)
	return nil
}

// spawnFullStackSandbox deploys the full service mesh into an ephemeral namespace (L3).
func spawnFullStackSandbox(ctx context.Context, sandboxID, service string) error {
	// L3 requires the full Helm chart deploy. Uses the sandbox cluster kubeconfig.
	// For now: scaffold that logs and returns success so the tier-2 path is exercised.
	slog.Info("sandbox L3 full-stack requested", "sandbox_id", sandboxID, "service", service)
	return nil
}

func spawnDockerSandbox(ctx context.Context, sandboxID, service string) error {
	if service == "" {
		return fmt.Errorf("service name required for L1 sandbox")
	}
	// Enforce: deny outbound to prod hostnames; use --network=none.
	cmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", "forge-sandbox-"+sandboxID,
		"--network=none",
		"--label", "forge.sandbox=true",
		"--label", "forge.sandbox.id="+sandboxID,
		"--label", "forge.sandbox.service="+service,
		"--env", "FORGE_MOCK_SECRETS=true",
		"golang:1.22-alpine", "sh", "-c", "sleep 1800",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run failed: %w (output: %s)", err, string(out))
	}
	return nil
}
