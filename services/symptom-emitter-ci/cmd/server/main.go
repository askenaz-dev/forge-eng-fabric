// symptom-emitter-ci is a GitHub Actions webhook receiver that normalises
// check_run and workflow_run failure events to symptom events on forge.symptoms.v1.
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/forge-eng-fabric/services/symptom-emitter-ci/internal/webhook"
)

const topic = "forge.symptoms.v1"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	addr := env("ADDR", ":9092")
	kafkaBrokers := env("KAFKA_BROKERS", "localhost:9094")
	webhookSecret := env("GITHUB_WEBHOOK_SECRET", "")

	client, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBrokers),
		kgo.DefaultProduceTopic(topic),
	)
	if err != nil {
		log.Fatalf("kafka client: %v", err)
	}
	defer client.Close()

	producer := &kafkaProducer{client: client}
	handler := webhook.New(webhookSecret, producer)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.Handle("/webhooks/github", handler)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	slog.Info("symptom-emitter-ci started", "addr", addr, "kafka", kafkaBrokers)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	<-done
}

type kafkaProducer struct {
	client *kgo.Client
}

func (p *kafkaProducer) Publish(ctx context.Context, key string, value []byte) error {
	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	}
	return p.client.ProduceSync(ctx, record).FirstErr()
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
