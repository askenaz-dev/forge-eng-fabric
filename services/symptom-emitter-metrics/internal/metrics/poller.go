// Package metrics polls Prometheus for threshold-cross signals and emits
// symptom events on state change (crossing or recovering from a threshold).
package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Producer publishes symptom events to Kafka.
type Producer interface {
	Publish(ctx context.Context, key string, value []byte) error
}

// Rule is a single Prometheus threshold rule.
type Rule struct {
	Name        string `yaml:"name"`
	Service     string `yaml:"service"`
	Signal      string `yaml:"signal"`
	Severity    string `yaml:"severity"`
	PromQL      string `yaml:"promql"`
	Description string `yaml:"description"`
}

// Config is the threshold rules configuration.
type Config struct {
	Cadence  string `yaml:"cadence"`
	Debounce string `yaml:"debounce"`
	Rules    []Rule `yaml:"rules"`
}

// Poller queries Prometheus and emits symptom events on threshold transitions.
type Poller struct {
	cfg        Config
	promURL    string
	producer   Producer
	mu         sync.Mutex
	state      map[string]bool // rule name → currently firing
}

// NewPoller creates a Poller from YAML config bytes.
func NewPoller(cfgBytes []byte, promURL string, p Producer) (*Poller, error) {
	var cfg Config
	if err := yaml.Unmarshal(cfgBytes, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &Poller{
		cfg:      cfg,
		promURL:  strings.TrimRight(promURL, "/"),
		producer: p,
		state:    make(map[string]bool),
	}, nil
}

// Run starts the polling loop until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	cadence := 60 * time.Second
	if d, err := time.ParseDuration(p.cfg.Cadence); err == nil {
		cadence = d
	}

	ticker := time.NewTicker(cadence)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	for _, rule := range p.cfg.Rules {
		firing, err := p.queryFiring(ctx, rule.PromQL)
		if err != nil {
			slog.Warn("prometheus query failed", "rule", rule.Name, "err", err)
			continue
		}
		p.mu.Lock()
		wasFiring := p.state[rule.Name]
		p.state[rule.Name] = firing
		p.mu.Unlock()

		if firing && !wasFiring {
			p.emit(ctx, rule, "threshold-crossed")
		} else if !firing && wasFiring {
			p.emit(ctx, rule, "threshold-recovered")
		}
	}
}

func (p *Poller) queryFiring(ctx context.Context, promQL string) (bool, error) {
	params := url.Values{"query": {promQL}}
	reqURL := p.promURL + "/api/v1/query?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string          `json:"resultType"`
			Result     json.RawMessage `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}
	if result.Status != "success" {
		return false, fmt.Errorf("prometheus status: %s", result.Status)
	}

	// A non-empty result means the threshold is currently exceeded.
	var rows []json.RawMessage
	if err := json.Unmarshal(result.Data.Result, &rows); err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

type symptomEvent struct {
	SymptomID       string `json:"symptom_id"`
	Fingerprint     string `json:"fingerprint"`
	Signal          string `json:"signal"`
	Service         string `json:"service"`
	Severity        string `json:"severity"`
	Emitter         string `json:"emitter"`
	ObservedAt      string `json:"observed_at"`
	SchemaVersion   string `json:"schema_version"`
	EvidenceExcerpt string `json:"evidence_excerpt"`
}

func (p *Poller) emit(ctx context.Context, rule Rule, suffix string) {
	signal := rule.Signal
	if suffix == "threshold-recovered" {
		signal = rule.Signal + "-recovered"
	}

	fingerprint := fmt.Sprintf("service:%s|signal:%s", rule.Service, signal)

	evt := symptomEvent{
		SymptomID:       uuid.NewString(),
		Fingerprint:     fingerprint,
		Signal:          signal,
		Service:         rule.Service,
		Severity:        rule.Severity,
		Emitter:         "symptom-emitter-metrics",
		ObservedAt:      time.Now().UTC().Format(time.RFC3339),
		SchemaVersion:   "v1",
		EvidenceExcerpt: fmt.Sprintf("rule=%s: %s", rule.Name, rule.Description),
	}

	b, err := json.Marshal(evt)
	if err != nil {
		slog.Error("marshal symptom event", "err", err)
		return
	}
	if err := p.producer.Publish(ctx, fingerprint, b); err != nil {
		slog.Error("publish symptom event", "fingerprint", fingerprint, "err", err)
		return
	}
	slog.Info("symptom emitted",
		"rule", rule.Name,
		"signal", signal,
		"service", rule.Service,
		"symptom_id", evt.SymptomID,
	)
}
