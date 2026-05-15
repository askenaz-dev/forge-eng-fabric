// symptom-emitter-probe checks HTTP/TCP/gRPC endpoints and emits probe-failed /
// probe-recovered SymptomEventV1 events when state changes are detected.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/forge-eng-fabric/services/symptom-emitter-probe/internal/prober"
	"github.com/forge-eng-fabric/services/symptom-emitter-probe/internal/registry"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	kafkaBrokers := env("KAFKA_BROKERS", "localhost:29092")
	symptomsTopic := env("SYMPTOMS_TOPIC", "forge.symptoms.v1")
	configPath := env("PROBES_CONFIG", "config/probes.yaml")

	cfg, err := registry.Load(configPath)
	if err != nil {
		slog.Error("failed to load probes config", "err", err)
		os.Exit(1)
	}

	kc, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(kafkaBrokers, ",")...))
	if err != nil {
		slog.Error("kafka client", "err", err)
		os.Exit(1)
	}
	defer kc.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("symptom-emitter-probe started",
		"probes", len(cfg.Probes),
		"cadence", cfg.Cadence,
		"debounce", cfg.Debounce,
	)

	runner := prober.New(cfg, kc, symptomsTopic)
	runner.Run(ctx)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
