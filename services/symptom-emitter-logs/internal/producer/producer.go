package producer

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// SymptomEvent is the wire format for forge.symptoms.v1.
type SymptomEvent struct {
	SymptomID       string    `json:"symptom_id"`
	Fingerprint     string    `json:"fingerprint"`
	Signal          string    `json:"signal"`
	Service         string    `json:"service"`
	Severity        string    `json:"severity"`
	Emitter         string    `json:"emitter"`
	TenantID        string    `json:"tenant_id,omitempty"`
	WorkspaceID     string    `json:"workspace_id,omitempty"`
	ErrorClass      string    `json:"error_class,omitempty"`
	Port            int       `json:"port,omitempty"`
	Route           string    `json:"route,omitempty"`
	EvidenceExcerpt string    `json:"evidence_excerpt"`
	EvidenceRef     string    `json:"evidence_ref,omitempty"`
	ObservedAt      time.Time `json:"observed_at"`
	SchemaVersion   string    `json:"schema_version"`
}

// Producer wraps a Kafka client with a local ring buffer and overflow counter.
type Producer struct {
	client    *kgo.Client
	topic     string
	buf       chan *SymptomEvent
	dropped   atomic.Int64
	done      chan struct{}
}

const bufSize = 512

// New creates a Producer and starts the background drain loop.
func New(client *kgo.Client, topic string) *Producer {
	p := &Producer{
		client: client,
		topic:  topic,
		buf:    make(chan *SymptomEvent, bufSize),
		done:   make(chan struct{}),
	}
	go p.drain()
	return p
}

// Emit queues an event. If the local buffer is full, the event is dropped and
// the overflow counter is incremented.
func (p *Producer) Emit(evt *SymptomEvent) {
	select {
	case p.buf <- evt:
	default:
		p.dropped.Add(1)
		slog.Warn("symptom-emitter-logs: buffer full, event dropped",
			"fingerprint", evt.Fingerprint,
			"total_dropped", p.dropped.Load(),
		)
	}
}

// DroppedCount returns the cumulative number of events dropped due to buffer overflow.
func (p *Producer) DroppedCount() int64 { return p.dropped.Load() }

// Close drains remaining events and waits for the loop to exit.
func (p *Producer) Close() {
	close(p.buf)
	<-p.done
}

func (p *Producer) drain() {
	defer close(p.done)
	for evt := range p.buf {
		p.publish(evt)
	}
}

func (p *Producer) publish(evt *SymptomEvent) {
	evt.SchemaVersion = "v1"
	body, err := json.Marshal(evt)
	if err != nil {
		slog.Error("symptom-emitter-logs: marshal error", "err", err)
		return
	}
	rec := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(evt.Fingerprint),
		Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "ce_type", Value: []byte("forge.symptom.v1")},
			{Key: "ce_specversion", Value: []byte("1.0")},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.client.ProduceSync(ctx, rec).FirstErr(); err != nil {
		slog.Error("symptom-emitter-logs: kafka publish failed", "err", err, "fingerprint", evt.Fingerprint)
	}
}
