// trigger-router is the platform service that subscribes to external
// sources (webhooks, cron, event-bus topics, IMAP mailboxes) and
// dispatches workflow-runtime executions when a trigger fires.
//
// See openspec/specs/automation-triggers and openspec/changes/ai-flow-
// authoring for the contract this service implements.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/forge-eng-fabric/services/trigger-router/internal/trigger"
)

func main() {
	registry := trigger.NewRegistry()
	sink := trigger.NoopSink{} // production wires Pulsar/NATS

	runtimeURL := envOr("WORKFLOW_RUNTIME_URL", "http://localhost:8093")
	runtime := &trigger.HTTPRuntimeClient{
		BaseURL:     runtimeURL,
		HTTP:        &http.Client{Timeout: 10 * time.Second},
		MaxAttempts: 4,
	}
	dispatcher := &trigger.Dispatcher{
		Runtime: runtime,
		Sink:    sink,
		DLQ:     trigger.NoopDLQ{},
	}

	cron := trigger.NewCronScheduler(registry, dispatcher)
	cron.Start()
	defer cron.Stop()

	bus := trigger.NewChannelBus() // dev only — production wires Pulsar
	knownTopics := defaultKnownTopics()
	eventbus := trigger.NewEventBusRouter(registry, dispatcher, bus, knownTopics)

	emailPoller := trigger.NewEmailPoller(registry, dispatcher, trigger.NoopMailbox{})
	// 30s poll loop for the email mailbox(es).
	stopEmail := startTicker(30*time.Second, func() { _ = emailPoller.Tick(nil) })
	defer stopEmail()

	server := &trigger.Server{
		Registry:   registry,
		Dispatcher: dispatcher,
		Webhook: &trigger.WebhookHandler{
			Registry:   registry,
			Dispatcher: dispatcher,
			Secrets:    trigger.StaticSecrets{}, // production wires the secrets broker
		},
	}

	addr := envOr("TRIGGER_ROUTER_ADDR", ":8097")
	log.Printf("trigger-router: listening on %s (workflow-runtime=%s)", addr, runtimeURL)
	_ = cron.Refresh
	_ = eventbus.Refresh
	// Refresh hooks land when the workflow.published.v1 subscriber wires in.

	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func startTicker(d time.Duration, fn func()) func() {
	t := time.NewTicker(d)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-t.C:
				fn()
			case <-stop:
				t.Stop()
				return
			}
		}
	}()
	return func() { close(stop) }
}

// defaultKnownTopics mirrors pkg/workflow/lint.KnownEventTopics. Kept
// inline here to avoid a cross-module dep; the lint package is the
// source of truth, and a periodic test could enforce parity.
func defaultKnownTopics() map[string]struct{} {
	topics := []string{
		"workflow.published.v1",
		"workflow.started.v1",
		"workflow.completed.v1",
		"workflow.failed.v1",
		"workflow.step.started.v1",
		"workflow.step.completed.v1",
		"workflow.trigger.fired.v1",
		"workflow.trigger.dropped.v1",
		"workflow.trigger.failed.v1",
		"workflow.llm.budget_exhausted.v1",
		"deployment.completed.v1",
		"deployment.failed.v1",
		"incident.opened.v1",
		"incident.resolved.v1",
		"symptom.detected.v1",
		"github.push.v1",
		"github.pull_request.opened.v1",
		"github.pull_request.merged.v1",
	}
	out := map[string]struct{}{}
	for _, t := range topics {
		out[t] = struct{}{}
	}
	return out
}
