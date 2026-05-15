package tailer

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MatchRule defines a Loki log-pattern match that produces a symptom.
type MatchRule struct {
	Name       string `yaml:"name"`
	Service    string `yaml:"service"`
	LokiQuery  string `yaml:"loki_query"`
	Signal     string `yaml:"signal"`
	Severity   string `yaml:"severity"`
	ErrorClass string `yaml:"error_class,omitempty"`
	Route      string `yaml:"route,omitempty"`
}

// Config holds all match rules for the emitter.
type Config struct {
	LokiURL string      `yaml:"loki_url"`
	Rules   []MatchRule `yaml:"rules"`
}

// LoadConfig reads and parses the YAML config file.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("tailer: open config %q: %w", path, err)
	}
	defer f.Close()
	var cfg Config
	if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("tailer: parse config: %w", err)
	}
	return &cfg, nil
}
