package detection

import (
	"testing"
	"time"
)

func TestPrometheusNormalizationCreatesIncident(t *testing.T) {
	svc := NewService(NewStore(), &MemorySink{})
	w := PrometheusWebhook{
		Status: "firing",
		Alerts: []PrometheusAlert{
			{
				Status: "firing",
				Labels: map[string]string{
					"alertname": "HighErrorRate",
					"service":   "app-foo",
					"env":       "prod",
					"severity":  "critical",
				},
				Annotations: map[string]string{"description": "5xx > 5%"},
				StartsAt:    time.Now(),
			},
		},
	}
	created, dedup, err := svc.IngestPrometheus(w)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if created != 1 || dedup != 0 {
		t.Fatalf("expected 1 created, got created=%d dedup=%d", created, dedup)
	}
	sink := svc.Sink.(*MemorySink)
	if len(sink.Events) != 1 || sink.Events[0].Type != "incident.detected.v1" {
		t.Fatalf("unexpected events: %+v", sink.Events)
	}
	data := sink.Events[0].Data
	if data["service"] != "app-foo" || data["env"] != "prod" {
		t.Fatalf("missing service/env: %+v", data)
	}
}

func TestDedupWithinWindow(t *testing.T) {
	svc := NewService(NewStore(), &MemorySink{})
	now := time.Now()
	svc.Now = func() time.Time { return now }
	w := PrometheusWebhook{
		Alerts: []PrometheusAlert{
			{Status: "firing", Labels: map[string]string{"alertname": "HighErrorRate", "service": "svc-a", "env": "prod"}},
		},
	}
	if c, _, _ := svc.IngestPrometheus(w); c != 1 {
		t.Fatalf("first should create")
	}
	// Advance 2 minutes — within dedup window.
	svc.Now = func() time.Time { return now.Add(2 * time.Minute) }
	c, d, _ := svc.IngestPrometheus(w)
	if c != 0 || d != 1 {
		t.Fatalf("expected dedup, got c=%d d=%d", c, d)
	}
	// Advance 6 minutes — outside dedup window: still attached because incident is still open.
	// Spec says dedup window applies on UpdatedAt. Outside the window from last update,
	// a new incident IS created.
	svc.Now = func() time.Time { return now.Add(2*time.Minute + 6*time.Minute) }
	c, d, _ = svc.IngestPrometheus(w)
	if c != 1 || d != 0 {
		t.Fatalf("expected new incident after window: c=%d d=%d", c, d)
	}
}

func TestManualDeclare(t *testing.T) {
	svc := NewService(NewStore(), &MemorySink{})
	inc, err := svc.Declare(DeclareRequest{
		Service:     "checkout",
		Environment: "stage",
		Title:       "user-reported slowness",
		Actor:       "alice@example.com",
	})
	if err != nil {
		t.Fatalf("declare: %v", err)
	}
	if inc.Source != SourceManual {
		t.Fatalf("expected manual source")
	}
	sink := svc.Sink.(*MemorySink)
	if len(sink.Events) != 1 {
		t.Fatalf("expected 1 event")
	}
}

func TestInternalEventIngestion(t *testing.T) {
	svc := NewService(NewStore(), &MemorySink{})
	created, err := svc.IngestInternal(InternalEvent{
		Type:    "deployment.failed.v1",
		Subject: "deploy/app-foo",
		Data:    map[string]any{"service": "app-foo", "env": "stage", "description": "rollout aborted"},
		Time:    time.Now(),
	})
	if err != nil {
		t.Fatalf("ingest internal: %v", err)
	}
	if !created {
		t.Fatal("expected created")
	}
}

func TestUnknownInternalEventRejected(t *testing.T) {
	svc := NewService(NewStore(), &MemorySink{})
	_, err := svc.IngestInternal(InternalEvent{Type: "random.event.v1"})
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestResolveEmitsEvent(t *testing.T) {
	svc := NewService(NewStore(), &MemorySink{})
	inc, _ := svc.Declare(DeclareRequest{Service: "s", Environment: "e", Title: "x"})
	if _, err := svc.Resolve(inc.ID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	sink := svc.Sink.(*MemorySink)
	if len(sink.Events) != 2 || sink.Events[1].Type != "incident.resolved.v1" {
		t.Fatalf("expected detected+resolved, got %+v", sink.Events)
	}
}
