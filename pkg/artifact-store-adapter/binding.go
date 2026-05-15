package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Backend names the supported artifact stores. The DB constraint on
// artifact_store_binding (`backend` IN (...)) MUST stay in sync with
// these constants; the migration in db/migrations/registry/0007 is the
// source of truth.
const (
	BackendNexus           = "nexus"
	BackendArtifactory     = "artifactory"
	BackendGitHubPackages  = "github-packages-private"
	BackendCodeArtifact    = "codeartifact"
	BackendNPMPublic       = "npm-public"
)

// BindingConfig is the row read from artifact_store_binding plus the
// runtime dependencies the factory needs. Settings carry the
// backend-specific configuration (base URL, repository name,
// region, etc.) — drivers parse this map into their own typed config.
type BindingConfig struct {
	TenantID      string
	Backend       string
	CredentialRef string
	Settings      map[string]any
}

// Factory builds an Adapter for a single tenant binding. Drivers register
// themselves at init time; the factory dispatches by Backend.
type Factory struct {
	drivers       map[string]DriverFactory
	secretFetcher SecretFetcher
	auditSink     AuditSink
}

// DriverFactory constructs a single Adapter instance. Each backend
// implementation registers one of these via RegisterDriver during init.
type DriverFactory func(ctx context.Context, cfg BindingConfig, deps Deps) (Adapter, error)

// Deps holds the shared dependencies the factory threads into every
// driver. Separated from BindingConfig so drivers can be tested by
// instantiating them directly with synthetic deps.
type Deps struct {
	SecretFetcher SecretFetcher
	AuditSink     AuditSink
}

var globalRegistry = map[string]DriverFactory{}

// RegisterDriver wires a driver into the global registry. Called from each
// driver's init() function. Re-registering a backend panics; this is a
// build-time error.
func RegisterDriver(backend string, f DriverFactory) {
	if _, exists := globalRegistry[backend]; exists {
		panic("adapter: backend already registered: " + backend)
	}
	globalRegistry[backend] = f
}

// NewFactory constructs a Factory wired to the global driver registry.
func NewFactory(secrets SecretFetcher, audit AuditSink) *Factory {
	if secrets == nil {
		secrets = StaticSecretFetcher{}
	}
	if audit == nil {
		audit = NoOpAuditSink{}
	}
	drivers := make(map[string]DriverFactory, len(globalRegistry))
	for k, v := range globalRegistry {
		drivers[k] = v
	}
	return &Factory{drivers: drivers, secretFetcher: secrets, auditSink: audit}
}

// RegisterDriverOn lets tests register a driver into a private Factory
// without polluting the global registry. Production code uses init() +
// RegisterDriver instead.
func (f *Factory) RegisterDriverOn(backend string, df DriverFactory) {
	f.drivers[backend] = df
}

// Build instantiates the Adapter for the given binding. It refuses public
// storage backends both by name (npm-public) and by Health probe (any driver
// whose Health reports IsPublicStorage=true). The Health probe runs
// synchronously before the adapter is returned, so a misconfigured backend
// cannot silently route bytes through a public registry.
//
// IsPublicOrigin on the Health result is not gated here: public-origin assets
// (e.g. mirrored npm packages) are explicitly allowed to be stored in private
// backends. The mirror flow sets the is_public_origin column on the asset row.
func (f *Factory) Build(ctx context.Context, cfg BindingConfig) (Adapter, error) {
	if cfg.Backend == BackendNPMPublic {
		return nil, &Error{
			Code:    ErrCodePublicBackend,
			Message: "npm-public is not a permitted backend; configure a private store (nexus, artifactory, github-packages-private, codeartifact)",
		}
	}
	driver, ok := f.drivers[cfg.Backend]
	if !ok {
		return nil, &Error{
			Code:    ErrCodeInvalidInput,
			Message: "unknown backend: " + cfg.Backend,
		}
	}
	a, err := driver(ctx, cfg, Deps{SecretFetcher: f.secretFetcher, AuditSink: f.auditSink})
	if err != nil {
		return nil, err
	}
	h, herr := a.Health(ctx)
	if herr != nil {
		return nil, fmt.Errorf("adapter: health probe failed for backend=%s: %w", cfg.Backend, herr)
	}
	if h.IsPublicStorage {
		return nil, &Error{
			Code:    ErrCodePublicBackend,
			Message: fmt.Sprintf("backend=%s reports is_public_storage=true; refusing to use a publicly accessible backend for skill artifacts", cfg.Backend),
		}
	}
	if !h.Healthy {
		return nil, &Error{
			Code:    ErrCodeBackendError,
			Message: fmt.Sprintf("backend=%s reports unhealthy: %s", cfg.Backend, h.Detail),
		}
	}
	return a, nil
}

// DecodeSettings is a small helper drivers use to map their typed config
// out of the BindingConfig.Settings map. Errors are returned with
// ErrCodeInvalidInput so the binding-time failure mode is uniform.
func DecodeSettings(in map[string]any, out any) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return &Error{Code: ErrCodeInvalidInput, Message: "encode settings: " + err.Error(), Wrapped: err}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &Error{Code: ErrCodeInvalidInput, Message: "decode settings: " + err.Error(), Wrapped: err}
	}
	return nil
}

// Errors used during binding setup.
var (
	ErrBackendNotRegistered = errors.New("backend not registered")
)
