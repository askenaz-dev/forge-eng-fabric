// Package holds implements the legal-hold mechanism for the data-retention
// policy (platform-gaps-closure 7.7). Holds pause retention deletions for
// tagged objects regardless of policy expiry, with auditable hold-set and
// hold-release operations.
package holds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Hold struct {
	ID         string          `json:"id"`
	Scope      string          `json:"scope"`
	ScopeID    string          `json:"scope_id"`
	DataType   string          `json:"data_type"`
	Selector   json.RawMessage `json:"selector"`
	Reason     string          `json:"reason"`
	Approver   string          `json:"approver"`
	SetAt      time.Time       `json:"set_at"`
	ReleasedAt *time.Time      `json:"released_at,omitempty"`
	ReleasedBy *string         `json:"released_by,omitempty"`
	ExpiresAt  *time.Time      `json:"expires_at,omitempty"`
}

type SetRequest struct {
	Scope     string          `json:"scope"`
	ScopeID   string          `json:"scope_id"`
	DataType  string          `json:"data_type"`
	Selector  json.RawMessage `json:"selector"`
	Reason    string          `json:"reason"`
	Approver  string          `json:"approver"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
}

type ReleaseRequest struct {
	Reason   string `json:"reason"`
	Approver string `json:"approver"`
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) Set(ctx context.Context, req SetRequest) (*Hold, error) {
	if req.Scope == "" || req.ScopeID == "" || req.DataType == "" || req.Reason == "" || req.Approver == "" {
		return nil, errors.New("scope, scope_id, data_type, reason, approver are required")
	}
	if !validScope(req.Scope) {
		return nil, fmt.Errorf("invalid scope: %s", req.Scope)
	}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO retention_legal_hold (scope, scope_id, data_type, selector, reason, approver, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, set_at`,
		req.Scope, req.ScopeID, req.DataType, req.Selector, req.Reason, req.Approver, req.ExpiresAt,
	)
	hold := &Hold{
		Scope:    req.Scope,
		ScopeID:  req.ScopeID,
		DataType: req.DataType,
		Selector: req.Selector,
		Reason:   req.Reason,
		Approver: req.Approver,
		ExpiresAt: req.ExpiresAt,
	}
	if err := row.Scan(&hold.ID, &hold.SetAt); err != nil {
		return nil, err
	}
	return hold, nil
}

func (s *Service) Release(ctx context.Context, id string, req ReleaseRequest) (*Hold, error) {
	if req.Reason == "" || req.Approver == "" {
		return nil, errors.New("reason and approver are required")
	}
	row := s.pool.QueryRow(ctx,
		`UPDATE retention_legal_hold
		    SET released_at = now(), released_by = $2
		  WHERE id = $1 AND released_at IS NULL
		  RETURNING id, scope, scope_id, data_type, selector, reason, approver, set_at, released_at, released_by, expires_at`,
		id, req.Approver,
	)
	h := &Hold{}
	if err := row.Scan(&h.ID, &h.Scope, &h.ScopeID, &h.DataType, &h.Selector, &h.Reason, &h.Approver, &h.SetAt, &h.ReleasedAt, &h.ReleasedBy, &h.ExpiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("hold not found or already released")
		}
		return nil, err
	}
	return h, nil
}

func (s *Service) ListActive(ctx context.Context, dataType string) ([]Hold, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, scope, scope_id, data_type, selector, reason, approver, set_at, released_at, released_by, expires_at
		   FROM retention_legal_hold
		  WHERE data_type = $1 AND released_at IS NULL`,
		dataType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Hold{}
	for rows.Next() {
		h := Hold{}
		if err := rows.Scan(&h.ID, &h.Scope, &h.ScopeID, &h.DataType, &h.Selector, &h.Reason, &h.Approver, &h.SetAt, &h.ReleasedAt, &h.ReleasedBy, &h.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, nil
}

func validScope(s string) bool {
	switch s {
	case "tenant", "business_unit", "workspace":
		return true
	}
	return false
}

// HTTP handlers -------------------------------------------------------------

type Handler struct{ Service *Service }

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/retention/holds", h.set)
	mux.HandleFunc("POST /v1/retention/holds/", h.release)
	mux.HandleFunc("GET /v1/retention/holds", h.list)
}

func (h *Handler) set(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req SetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	hold, err := h.Service.Set(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusCreated, hold)
}

func (h *Handler) release(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		// Fall through to the older path syntax `/v1/retention/holds/<id>/release`.
		// Strip the `/v1/retention/holds/` prefix manually.
		id = stripIDFromPath(r.URL.Path)
	}
	defer r.Body.Close()
	var req ReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	hold, err := h.Service.Release(r.Context(), id, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, hold)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	dataType := r.URL.Query().Get("data_type")
	if dataType == "" {
		dataType = "audit_event"
	}
	holds, err := h.Service.ListActive(r.Context(), dataType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"holds": holds})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func stripIDFromPath(p string) string {
	const prefix = "/v1/retention/holds/"
	if len(p) <= len(prefix) {
		return ""
	}
	rest := p[len(prefix):]
	// Accept both `<id>` and `<id>/release`.
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' {
			return rest[:i]
		}
	}
	return rest
}
