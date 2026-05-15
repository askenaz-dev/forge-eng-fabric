package registry

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ProbeKind enumerates supported probe types.
type ProbeKind string

const (
	ProbeHTTP ProbeKind = "http"
	ProbeTCP  ProbeKind = "tcp"
	ProbeGRPC ProbeKind = "grpc"
)

// Probe defines a single health-check target.
type Probe struct {
	Name     string    `yaml:"name"`
	Kind     ProbeKind `yaml:"kind"`
	URL      string    `yaml:"url,omitempty"`
	Addr     string    `yaml:"addr,omitempty"`
	Service  string    `yaml:"service"`
	Severity string    `yaml:"severity"`
}

// Config holds the full probe registry.
type Config struct {
	Cadence  time.Duration `yaml:"cadence"`
	Debounce int           `yaml:"debounce"`
	Probes   []Probe       `yaml:"probes"`
}

// Load reads and parses the probe registry YAML.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("registry: open %q: %w", path, err)
	}
	defer f.Close()
	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("registry: decode: %w", err)
	}
	if cfg.Cadence == 0 {
		cfg.Cadence = 30 * time.Second
	}
	if cfg.Debounce == 0 {
		cfg.Debounce = 2
	}
	return &cfg, nil
}
