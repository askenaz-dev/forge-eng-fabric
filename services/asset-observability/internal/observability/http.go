package observability

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
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
	mux.HandleFunc("POST /v1/gateway/installs", h.ingestInstall)
	mux.HandleFunc("GET /v1/assets/", h.metrics)
	mux.HandleFunc("GET /v1/services/health", h.servicesHealth)
	mux.HandleFunc("GET /v1/alfred/agent-mode/metrics", h.agentModeMetrics)
}

// agentModeMetrics returns the per-workspace Alfred agent-mode rollup used by
// the portal dashboard tile (cost p95, success rate, HITL-pause rate).
func (h *Handler) agentModeMetrics(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(h.Service.AgentModeMetrics(workspaceID))
}

// knownService describes a service node that the dashboard mesh visualises.
// The id matches the layout positions in the portal's ServicesMeshPanel; the
// probe is a TCP dial so the state reflects real reachability rather than a
// canned value.
type knownService struct {
	ID    string
	Kind  string
	Probe string // host:port to TCP-dial; empty means always healthy (logical node).
}

var defaultServices = []knownService{
	{ID: "orchestrator", Kind: "orchestration", Probe: ""},
	{ID: "policy-svc", Kind: "policy", Probe: "localhost:8181"},
	{ID: "openfga", Kind: "authz", Probe: "localhost:8088"},
	{ID: "registry", Kind: "registry", Probe: "localhost:8089"},
	{ID: "audit", Kind: "audit", Probe: "localhost:8083"},
	{ID: "workflow-runtime", Kind: "runtime", Probe: "localhost:8093"},
	// context-eng is a logical architectural node (RAG / context engine) with
	// no dedicated process in dev — reported as a logical node so the mesh
	// reflects intent without lying about a missing service being "down".
	{ID: "context-eng", Kind: "context", Probe: ""},
	{ID: "spec-engine", Kind: "spec", Probe: "localhost:8094"},
	{ID: "pgvector", Kind: "data", Probe: "localhost:15432"},
}

type serviceHealth struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	State string `json:"state"`
}

func (h *Handler) servicesHealth(w http.ResponseWriter, r *http.Request) {
	const probeTimeout = 400 * time.Millisecond
	results := make([]serviceHealth, len(defaultServices))
	var wg sync.WaitGroup
	for i, svc := range defaultServices {
		i, svc := i, svc
		if svc.Probe == "" {
			results[i] = serviceHealth{ID: svc.ID, Kind: svc.Kind, State: "healthy"}
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", svc.Probe, probeTimeout)
			state := "down"
			if err == nil {
				state = "healthy"
				_ = conn.Close()
			}
			results[i] = serviceHealth{ID: svc.ID, Kind: svc.Kind, State: state}
		}()
	}
	wg.Wait()
	writeJSON(w, http.StatusOK, map[string]any{"services": results})
}

func (h *Handler) ingestInstall(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var in Install
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.Service.Store.RecordInstall(in)
	w.WriteHeader(http.StatusAccepted)
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
	source := q.Get("source")
	var series *MetricSeries
	var err error
	if source != "" {
		series, err = h.Service.Store.Aggregate(id, rng, gran, time.Now().UTC(), source)
	} else {
		series, err = h.Service.Store.Aggregate(id, rng, gran, time.Now().UTC())
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if series.DriftAlert {
		_ = h.Service.Sink.Emit(newEvent("", "", "asset.eval.drift.detected.v1", "asset/"+id, map[string]any{
			"asset_id":     id,
			"range":        rng,
			"drift_source": series.DriftSource,
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
