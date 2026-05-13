// skill-gateway is the public developer-facing surface for Forge assets. See
// openspec/changes/add-developer-skill-gateway for the spec.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/forge-eng-fabric/services/skill-gateway/internal/packagestore"
	"github.com/forge-eng-fabric/services/skill-gateway/internal/registry"
	"github.com/forge-eng-fabric/services/skill-gateway/internal/server"
	"github.com/forge-eng-fabric/services/skill-gateway/internal/telemetry"
)

func main() {
	cfg := loadConfig()
	shutdownTelemetry, err := telemetry.Init(context.Background(), "skill-gateway", cfg.Environment, cfg.OTLPEndpoint)
	if err != nil {
		log.Fatalf("otel: %v", err)
	}
	defer func() { _ = shutdownTelemetry(context.Background()) }()

	pool, err := pgxpool.New(context.Background(), cfg.PostgresURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer pool.Close()

	var rdb *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			log.Fatalf("redis url: %v", err)
		}
		rdb = redis.NewClient(opt)
		defer rdb.Close()
	}

	kc, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(cfg.KafkaBrokers, ",")...), kgo.AllowAutoTopicCreation())
	if err != nil {
		log.Fatalf("kafka: %v", err)
	}
	defer kc.Close()

	srv := server.New(server.Config{
		Addr:           cfg.Addr,
		Pool:           pool,
		Redis:          rdb,
		Kafka:          kc,
		EventsTopic:    cfg.EventsTopic,
		Registry:       registry.NewClient(cfg.RegistryURL, cfg.RegistrySystemToken),
		PackageStore:   packagestore.NewMemoryStore(), // wire to S3 in prod
		AllowedOrigins: splitCSV(cfg.AllowedOrigins),
		RateCapacity:   cfg.RateCapacity,
		RateWindow:     cfg.RateWindow,
	})

	httpSrv := &http.Server{Addr: cfg.Addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("skill-gateway listening on %s", cfg.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

type config struct {
	Addr                string
	PostgresURL         string
	RedisURL            string
	KafkaBrokers        string
	EventsTopic         string
	RegistryURL         string
	RegistrySystemToken string
	AllowedOrigins      string
	Environment         string
	OTLPEndpoint        string
	RateCapacity        int
	RateWindow          time.Duration
}

func loadConfig() config {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	getInt := func(k string, def int) int {
		v := os.Getenv(k)
		if v == "" {
			return def
		}
		var n int
		for _, c := range v {
			if c < '0' || c > '9' {
				return def
			}
			n = n*10 + int(c-'0')
		}
		return n
	}
	getDur := func(k string, def time.Duration) time.Duration {
		if v := os.Getenv(k); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				return d
			}
		}
		return def
	}
	return config{
		Addr:                get("ADDR", ":8120"),
		PostgresURL:         get("POSTGRES_URL", "postgres://forge:forge@localhost:15432/forge_registry?sslmode=disable"),
		RedisURL:            get("REDIS_URL", ""),
		KafkaBrokers:        get("KAFKA_BROKERS", "localhost:29092"),
		EventsTopic:         get("EVENTS_TOPIC", "forge.events"),
		RegistryURL:         get("REGISTRY_URL", "http://localhost:8082"),
		RegistrySystemToken: get("REGISTRY_SYSTEM_TOKEN", ""),
		AllowedOrigins:      get("ALLOWED_ORIGINS", ""),
		Environment:         get("ENV", "local"),
		OTLPEndpoint:        get("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		RateCapacity:        getInt("RATE_CAPACITY", 60),
		RateWindow:          getDur("RATE_WINDOW", time.Minute),
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
