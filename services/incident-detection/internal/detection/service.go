package detection

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service binds the store and event sink.
type Service struct {
	Store *Store
	Sink  Sink
	Now   func() time.Time
}

// NewService creates a Service with sensible defaults.
func NewService(store *Store, sink Sink) *Service {
	if sink == nil {
		sink = &MemorySink{}
	}
	return &Service{Store: store, Sink: sink, Now: func() time.Time { return time.Now().UTC() }}
}

// IngestPrometheus normalizes and stores a Prometheus alert webhook batch.
// Returns the count of new incidents and updated incidents.
func (s *Service) IngestPrometheus(w PrometheusWebhook) (newCount, dedupCount int, err error) {
	for _, a := range w.Alerts {
		inc, ev, err := fromPrometheus(a)
		if err != nil {
			continue
		}
		_, created := s.Store.upsert(inc, ev, s.Now())
		if created {
			s.emitDetected(inc)
			newCount++
		} else {
			dedupCount++
		}
	}
	return newCount, dedupCount, nil
}

// IngestCloudMonitoring normalizes and stores a Cloud Monitoring webhook.
func (s *Service) IngestCloudMonitoring(w CloudMonitoringWebhook) (bool, error) {
	inc, ev, err := fromCloudMonitoring(w.Incident)
	if err != nil {
		return false, err
	}
	_, created := s.Store.upsert(inc, ev, s.Now())
	if created {
		s.emitDetected(inc)
	}
	return created, nil
}

// IngestLoki uses the same wire format as Prometheus.
func (s *Service) IngestLoki(w LokiWebhook) (newCount, dedupCount int, err error) {
	for _, a := range w.Alerts {
		a.Labels["source_kind"] = "loki"
		inc, ev, err := fromPrometheus(a)
		if err != nil {
			continue
		}
		inc.Source = SourceLoki
		ev.Source = SourceLoki
		_, created := s.Store.upsert(inc, ev, s.Now())
		if created {
			s.emitDetected(inc)
			newCount++
		} else {
			dedupCount++
		}
	}
	return newCount, dedupCount, nil
}

// IngestInternal handles internal CloudEvents.
func (s *Service) IngestInternal(e InternalEvent) (bool, error) {
	inc, ev, err := fromInternal(e)
	if err != nil {
		return false, err
	}
	_, created := s.Store.upsert(inc, ev, s.Now())
	if created {
		s.emitDetected(inc)
	}
	return created, nil
}

// Declare creates an incident from a manual declaration.
func (s *Service) Declare(req DeclareRequest) (*Incident, error) {
	if req.Service == "" || req.Environment == "" {
		return nil, fmt.Errorf("%w: service and environment required", ErrInvalidPayload)
	}
	if req.Title == "" {
		return nil, fmt.Errorf("%w: title required", ErrInvalidPayload)
	}
	if req.Severity == "" {
		req.Severity = SeverityWarning
	}
	now := s.Now()
	sig := signatureHash(req.Service, req.Environment, req.Title)
	inc := &Incident{
		ID:            "inc-" + uuid.NewString(),
		TenantID:      req.TenantID,
		WorkspaceID:   req.WorkspaceID,
		Service:       req.Service,
		Environment:   req.Environment,
		SignatureHash: sig,
		Source:        SourceManual,
		Severity:      req.Severity,
		Title:         req.Title,
		Description:   req.Description,
		Labels:        req.Labels,
		Synthetic:     req.Synthetic,
	}
	ev := Event{
		ID:         "ie-" + uuid.NewString(),
		Source:     SourceManual,
		Severity:   req.Severity,
		OccurredAt: now,
		Payload:    map[string]any{"actor": req.Actor, "description": req.Description},
		Labels:     req.Labels,
	}
	stored, created := s.Store.upsert(inc, ev, now)
	if created {
		s.emitDetected(stored)
	}
	return stored, nil
}

// Resolve marks an incident as resolved and emits incident.resolved.v1.
func (s *Service) Resolve(id string) (*Incident, error) {
	inc, err := s.Store.Resolve(id, s.Now())
	if err != nil {
		return nil, err
	}
	_ = s.Sink.Emit(newEvent(inc.TenantID, inc.WorkspaceID, "incident.resolved.v1",
		"incident/"+inc.ID, map[string]any{
			"incident_id": inc.ID,
			"service":     inc.Service,
			"env":         inc.Environment,
			"resolved_at": inc.ResolvedAt,
		}))
	return inc, nil
}

func (s *Service) emitDetected(inc *Incident) {
	if errors.Is(s.Sink.Emit(newEvent(inc.TenantID, inc.WorkspaceID, "incident.detected.v1",
		"incident/"+inc.ID, map[string]any{
			"incident_id":    inc.ID,
			"service":        inc.Service,
			"env":            inc.Environment,
			"signature_hash": inc.SignatureHash,
			"severity":       inc.Severity,
			"source":         inc.Source,
			"title":          inc.Title,
			"synthetic":      inc.Synthetic,
		})), nil) {
		// best effort
	}
}
