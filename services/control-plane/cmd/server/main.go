// Control-plane: Tenant / BusinessUnit / Workspace CRUD for Phase 0.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/forge-eng-fabric/services/control-plane/internal/auth"
	"github.com/forge-eng-fabric/services/control-plane/internal/events"
	"github.com/forge-eng-fabric/services/control-plane/internal/httpx"
	"github.com/forge-eng-fabric/services/control-plane/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg := loadConfig()

	db, err := store.Open(context.Background(), cfg.PostgresURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	verifier, err := auth.NewKeycloakVerifier(cfg.KeycloakIssuer, cfg.KeycloakAudience)
	if err != nil {
		log.Fatalf("keycloak: %v", err)
	}

	fga, err := auth.NewOpenFGAClient(cfg.OpenFGAURL, cfg.OpenFGAStore, cfg.OpenFGAModel)
	if err != nil {
		log.Fatalf("openfga: %v", err)
	}

	pub, err := events.NewKafkaPublisher(cfg.KafkaBrokers, cfg.EventsTopic)
	if err != nil {
		log.Fatalf("kafka: %v", err)
	}
	defer pub.Close()

	api := httpx.NewAPI(db, fga, pub)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(httpx.CorrelationID)
	r.Use(httpx.AccessLog)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "db not ready", 503)
			return
		}
		w.WriteHeader(200)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(verifier))
		api.Routes(r)
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("control-plane listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

type config struct {
	Addr             string
	PostgresURL      string
	KeycloakIssuer   string
	KeycloakAudience string
	OpenFGAURL       string
	OpenFGAStore     string
	OpenFGAModel     string
	KafkaBrokers     string
	EventsTopic      string
}

func loadConfig() config {
	get := func(k, def string) string {
		if v := os.Getenv(k); v != "" {
			return v
		}
		return def
	}
	return config{
		Addr:             get("ADDR", ":8081"),
		PostgresURL:      get("POSTGRES_URL", "postgres://forge:forge@localhost:5432/forge_control_plane?sslmode=disable"),
		KeycloakIssuer:   get("KEYCLOAK_ISSUER", "http://localhost:8080/realms/forge"),
		KeycloakAudience: get("KEYCLOAK_AUDIENCE", "forge-control-plane"),
		OpenFGAURL:       get("OPENFGA_API_URL", "http://localhost:8088"),
		OpenFGAStore:     get("OPENFGA_STORE_ID", ""),
		OpenFGAModel:     get("OPENFGA_AUTHORIZATION_MODEL_ID", ""),
		KafkaBrokers:     get("KAFKA_BROKERS", "localhost:29092"),
		EventsTopic:      get("EVENTS_TOPIC", "forge.events"),
	}
}
