package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

// CloudEvent is our internal struct that we serialize as a CloudEvents 1.0
// JSON payload sent to Kafka in structured content mode.
type CloudEvent struct {
	Type          string    // com.forge.workspace.created.v1
	Source        string    // forge://service/control-plane
	Subject       string
	TenantID      string
	WorkspaceID   string
	Actor         string
	CorrelationID string
	Time          time.Time
	Data          any
}

// KafkaPublisher publishes CloudEvents to a single Kafka topic.
type KafkaPublisher struct {
	cl    *kgo.Client
	topic string
}

func NewKafkaPublisher(brokers, topic string) (*KafkaPublisher, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(splitCSV(brokers)...),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, err
	}
	return &KafkaPublisher{cl: cl, topic: topic}, nil
}

func (p *KafkaPublisher) Close() { p.cl.Close() }

func (p *KafkaPublisher) Publish(ctx context.Context, ev CloudEvent) error {
	if ev.Time.IsZero() {
		ev.Time = time.Now().UTC()
	}
	envelope := map[string]any{
		"specversion":        "1.0",
		"id":                 uuid.NewString(),
		"source":             ev.Source,
		"type":               ev.Type,
		"subject":            ev.Subject,
		"time":               ev.Time.Format(time.RFC3339Nano),
		"datacontenttype":    "application/json",
		"forgetenantid":      ev.TenantID,
		"forgeworkspaceid":   ev.WorkspaceID,
		"forgeactor":         ev.Actor,
		"forgecorrelationid": ev.CorrelationID,
		"data":               ev.Data,
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	rec := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(ev.TenantID),
		Value: body,
		Headers: []kgo.RecordHeader{
			{Key: "ce_type", Value: []byte(ev.Type)},
			{Key: "ce_source", Value: []byte(ev.Source)},
			{Key: "content-type", Value: []byte("application/cloudevents+json")},
		},
	}
	return p.cl.ProduceSync(ctx, rec).FirstErr()
}

func splitCSV(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
