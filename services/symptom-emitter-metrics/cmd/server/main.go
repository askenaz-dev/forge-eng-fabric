// symptom-emitter-metrics queries Prometheus for threshold-cross signals
// and emits symptom events to forge.symptoms.v1 on state transitions.
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/forge-eng-fabric/services/symptom-emitter-metrics/internal/metrics"
)

const topic = "forge.symptoms.v1"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	kafkaBrokers := env("KAFKA_BROKERS", "localhost:9094")
	promURL := env("PROMETHEUS_URL", "http://localhost:9090")
	configPath := env("CONFIG_PATH", "config/thresholds.yaml")

	cfgBytes, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("read config: %v", err)
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBrokers),
		kgo.DefaultProduceTopic(topic),
	)
	if err != nil {
		log.Fatalf("kafka client: %v", err)
	}
	defer client.Close()

	producer := &kafkaProducer{client: client}
	poller, err := metrics.NewPoller(cfgBytes, promURL, producer)
	if err != nil {
		log.Fatalf("poller init: %v", err)
	}

	slog.Info("symptom-emitter-metrics started", "kafka", kafkaBrokers, "prometheus", promURL)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	poller.Run(ctx)
}

type kafkaProducer struct {
	client *kgo.Client
}

func (p *kafkaProducer) Publish(ctx context.Context, key string, value []byte) error {
	record := &kgo.Record{Topic: topic, Key: []byte(key), Value: value}
	return p.client.ProduceSync(ctx, record).FirstErr()
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
