package observability

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Service binds the store and emits drift events.
type Service struct {
	Store *Store
	Sink  Sink
}

// NewService creates a Service.
func NewService(store *Store, sink Sink) *Service {
	if sink == nil {
		sink = &MemorySink{}
	}
	return &Service{Store: store, Sink: sink}
}

// Handler exposes endpoints over HTTP.
type Handler struct {
	Service *Service
}

// NewHandler builds the handler.
func NewHandler(s *Service) *Handler { return &Handler{Service: s} }

// Mount installs routes.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("POST /v1/assets/invocations", h.ingest)
	mux.HandleFunc("GET /v1/assets/", h.metrics)
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var inv Invocation
	if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.Service.Store.Ingest(inv)
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) metrics(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/assets/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "metrics" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	q := r.URL.Query()
	rng := MetricRange(defaultStr(q.Get("range"), "24h"))
	gran := Granularity(defaultStr(q.Get("granularity"), "hour"))
	series, err := h.Service.Store.Aggregate(id, rng, gran, time.Now().UTC())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if series.DriftAlert {
		_ = h.Service.Sink.Emit(newEvent("", "", "asset.eval.drift.detected.v1", "asset/"+id, map[string]any{
			"asset_id": id,
			"range":    rng,
		}))
	}
	writeJSON(w, http.StatusOK, series)
}

func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
