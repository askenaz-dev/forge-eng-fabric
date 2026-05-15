package prober

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/forge-eng-fabric/services/symptom-emitter-probe/internal/registry"
)

type state struct {
	up           bool
	failStreak   int
}

// Runner periodically checks probes and emits symptom events on state transitions.
type Runner struct {
	cfg    *registry.Config
	client *kgo.Client
	topic  string
	states map[string]*state
	http   *http.Client
}

// New creates a Runner.
func New(cfg *registry.Config, client *kgo.Client, topic string) *Runner {
	return &Runner{
		cfg:    cfg,
		client: client,
		topic:  topic,
		states: make(map[string]*state),
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Run starts the probe loop and blocks until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) {
	for _, p := range r.cfg.Probes {
		r.states[p.Name] = &state{up: true}
	}

	ticker := time.NewTicker(r.cfg.Cadence)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, p := range r.cfg.Probes {
				r.check(ctx, p)
			}
		}
	}
}

func (r *Runner) check(ctx context.Context, p registry.Probe) {
	ok := r.probe(ctx, p)
	s := r.states[p.Name]

	if !ok {
		s.failStreak++
		if s.up && s.failStreak >= r.cfg.Debounce {
			s.up = false
			r.emit(ctx, p, "probe-failed")
		}
		return
	}
	if !s.up {
		s.up = true
		s.failStreak = 0
		r.emit(ctx, p, "probe-recovered")
	} else {
		s.failStreak = 0
	}
}

func (r *Runner) probe(ctx context.Context, p registry.Probe) bool {
	switch p.Kind {
	case registry.ProbeHTTP:
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
		resp, err := r.http.Do(req)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode < 500
	case registry.ProbeTCP:
		d := &net.Dialer{Timeout: 3 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", p.Addr)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}
	return false
}

type symptomEvent struct {
	SymptomID       string    `json:"symptom_id"`
	Fingerprint     string    `json:"fingerprint"`
	Signal          string    `json:"signal"`
	Service         string    `json:"service"`
	Severity        string    `json:"severity"`
	Emitter         string    `json:"emitter"`
	EvidenceExcerpt string    `json:"evidence_excerpt"`
	ObservedAt      time.Time `json:"observed_at"`
	SchemaVersion   string    `json:"schema_version"`
}

func (r *Runner) emit(ctx context.Context, p registry.Probe, signal string) {
	fp := fmt.Sprintf("service:%s|signal:%s", p.Service, signal)
	evt := &symptomEvent{
		SymptomID:       uuid.NewString(),
		Fingerprint:     fp,
		Signal:          signal,
		Service:         p.Service,
		Severity:        p.Severity,
		Emitter:         "symptom-emitter-probe",
		EvidenceExcerpt: fmt.Sprintf("<evidence>\nprobe=%s kind=%s signal=%s\n</evidence>", p.Name, p.Kind, signal),
		ObservedAt:      time.Now().UTC(),
		SchemaVersion:   "v1",
	}
	body, _ := json.Marshal(evt)
	rec := &kgo.Record{
		Topic: r.topic,
		Key:   []byte(fp),
		Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "ce_type", Value: []byte("forge.symptom.v1")},
			{Key: "ce_specversion", Value: []byte("1.0")},
		},
	}
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := r.client.ProduceSync(ctx2, rec).FirstErr(); err != nil {
		slog.Error("symptom-emitter-probe: kafka publish failed", "err", err, "probe", p.Name)
		return
	}
	slog.Info("probe event emitted", "probe", p.Name, "signal", signal, "fingerprint", fp)
}
