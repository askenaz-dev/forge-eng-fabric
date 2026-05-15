package iac

import (
	"encoding/json"
	"net/http"
)

// Handler exposes the IaC skills over HTTP.
type Handler struct{ skills *IaCSkills }

// NewHandler wraps an IaCSkills instance.
func NewHandler(skills *IaCSkills) *Handler { return &Handler{skills: skills} }

// Mount installs routes onto mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /v1/skills/generate-terraform", h.handleGenerateTerraform)
	mux.HandleFunc("POST /v1/skills/generate-helm-values", h.handleGenerateHelmValues)
	mux.HandleFunc("POST /v1/skills/validate-iac", h.handleValidateIaC)
	mux.HandleFunc("POST /v1/skills/apply-iac", h.handleApplyIaC)
	// GitOps runner: called by the CI/CD pipeline on PR merge to trigger the actual apply.
	mux.HandleFunc("POST /v1/hooks/pr-merged", h.handlePRMerged)
}

func (h *Handler) handleGenerateTerraform(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var in GenerateTerraformInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := h.skills.GenerateTerraform(r.Context(), in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) handleGenerateHelmValues(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var in GenerateHelmInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := h.skills.GenerateHelmValues(r.Context(), in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) handleValidateIaC(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var in ValidateIaCInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := h.skills.ValidateIaC(r.Context(), in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) handleApplyIaC(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var in ApplyIaCInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := h.skills.ApplyIaC(r.Context(), in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) handlePRMerged(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var in PRMergedEvent
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out, err := h.skills.GitOpsApply(r.Context(), in)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
