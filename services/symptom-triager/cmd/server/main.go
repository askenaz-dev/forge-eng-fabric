// symptom-triager is the sole consumer of forge.symptoms.v1.
// It validates, deduplicates, applies noise rules, and spawns
// AgentModeSession with actor=system:alfred when triage rules fire.
// Session spawning is controlled by SESSION_SPAWNING_ENABLED (default false in iter 1).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/forge-eng-fabric/services/symptom-triager/internal/dlq"
	"github.com/forge-eng-fabric/services/symptom-triager/internal/spawner"
	"github.com/forge-eng-fabric/services/symptom-triager/internal/triage"
	"github.com/forge-eng-fabric/services/symptom-triager/internal/validator"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	kafkaBrokers := env("KAFKA_BROKERS", "localhost:29092")
	symptomsTopic := env("SYMPTOMS_TOPIC", "forge.symptoms.v1")
	dlqTopic := env("DLQ_TOPIC", "forge.symptoms.v1.dlq")
	alfredURL := env("ALFRED_URL", "http://localhost:8090")
	alfredToken := env("ALFRED_TOKEN", "")
	alfredWorkspace := env("ALFRED_WORKSPACE_ID", "")
	sessionSpawning := env("SESSION_SPAWNING_ENABLED", "false") == "true"
	consumerGroup := "forge-symptom-triager"

	kc, err := kgo.NewClient(
		kgo.SeedBrokers(strings.Split(kafkaBrokers, ",")...),
		kgo.ConsumerGroup(consumerGroup),
		kgo.ConsumeTopics(symptomsTopic),
	)
	if err != nil {
		slog.Error("kafka client", "err", err)
		os.Exit(1)
	}
	defer kc.Close()

	dlqProd := dlq.New(kc, dlqTopic)
	sp := spawner.New(alfredURL, alfredToken, sessionSpawning)
	engine := triage.NewEngine(sp, sessionSpawning, alfredWorkspace)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("symptom-triager started",
		"topic", symptomsTopic,
		"group", consumerGroup,
		"session_spawning", sessionSpawning,
	)

	for {
		fetches := kc.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				slog.Error("fetch error", "err", e.Err, "topic", e.Topic, "partition", e.Partition)
			}
		}

		fetches.EachRecord(func(rec *kgo.Record) {
			evt, err := validator.Validate(rec.Value)
			if err != nil {
				slog.Warn("symptom validation failed",
					"err", err,
					"key", string(rec.Key),
					"offset", rec.Offset,
				)
				dlqProd.Send(ctx, rec.Key, rec.Value, err.Error())
				return
			}
			_ = engine.Decide(ctx, evt)
		})
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
