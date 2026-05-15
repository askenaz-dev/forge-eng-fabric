// Package dlq provides a producer for the dead-letter queue topic.
package dlq

import (
	"context"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer sends rejected events to forge.symptoms.v1.dlq.
type Producer struct {
	client *kgo.Client
	topic  string
}

// New creates a DLQ Producer.
func New(client *kgo.Client, topic string) *Producer {
	return &Producer{client: client, topic: topic}
}

// Send routes raw event bytes to the DLQ with a validation_error header.
func (p *Producer) Send(ctx context.Context, key, value []byte, reason string) {
	rec := &kgo.Record{
		Topic: p.topic,
		Key:   key,
		Value: value,
		Headers: []kgo.RecordHeader{
			{Key: "validation_error", Value: []byte(reason)},
			{Key: "dlq_ts", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		},
	}
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := p.client.ProduceSync(ctx2, rec).FirstErr(); err != nil {
		slog.Error("dlq: failed to send", "err", err, "reason", reason)
	}
}
