// symptom-emitter-logs tails Loki, matches log patterns, and publishes
// normalised SymptomEventV1 messages to forge.symptoms.v1.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/forge-eng-fabric/services/symptom-emitter-logs/internal/fingerprint"
	"github.com/forge-eng-fabric/services/symptom-emitter-logs/internal/producer"
	"github.com/forge-eng-fabric/services/symptom-emitter-logs/internal/sanitiser"
	"github.com/forge-eng-fabric/services/symptom-emitter-logs/internal/tailer"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	kafkaBrokers := env("KAFKA_BROKERS", "localhost:29092")
	symptomsTopic := env("SYMPTOMS_TOPIC", "forge.symptoms.v1")
	configPath := env("RULES_CONFIG", "config/rules.yaml")

	cfg, err := tailer.LoadConfig(configPath)
	if err != nil {
		slog.Error("failed to load rules config", "err", err)
		os.Exit(1)
	}

	kc, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(kafkaBrokers, ",")...))
	if err != nil {
		slog.Error("kafka client", "err", err)
		os.Exit(1)
	}
	defer kc.Close()

	prod := producer.New(kc, symptomsTopic)
	defer prod.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("symptom-emitter-logs started",
		"loki_url", cfg.LokiURL,
		"rules", len(cfg.Rules),
		"topic", symptomsTopic,
	)

	// Polling loop: in a real implementation this would be a Loki tail websocket.
	// For the scaffold, we poll every 30s and emit synthetic events for each rule.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down")
			return
		case <-ticker.C:
			for _, rule := range cfg.Rules {
				fp, err := fingerprint.Build(fingerprint.Dims{
					"service":     rule.Service,
					"signal":      rule.Signal,
					"error_class": rule.ErrorClass,
				})
				if err != nil {
					slog.Warn("fingerprint build failed", "rule", rule.Name, "err", err)
					continue
				}
				excerpt := sanitiser.Sanitise("[match] " + rule.LokiQuery)
				evt := &producer.SymptomEvent{
					SymptomID:       uuid.NewString(),
					Fingerprint:     fp,
					Signal:          rule.Signal,
					Service:         rule.Service,
					Severity:        rule.Severity,
					Emitter:         "symptom-emitter-logs",
					ErrorClass:      rule.ErrorClass,
					EvidenceExcerpt: excerpt,
					ObservedAt:      time.Now().UTC(),
				}
				prod.Emit(evt)
				slog.Info("symptom emitted",
					"fingerprint", fp,
					"signal", rule.Signal,
					"rule", rule.Name,
				)
			}
			slog.Info("producer stats", "dropped_total", prod.DroppedCount())
		}
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
