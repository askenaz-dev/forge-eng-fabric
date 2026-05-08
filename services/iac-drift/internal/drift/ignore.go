package drift

import (
	"errors"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

var ErrSuppressionMissingExpiration = errors.New("suppression_missing_expiration")

type IgnoreFile struct {
	Suppressions []Suppression `yaml:"suppressions" json:"suppressions"`
}

type Suppression struct {
	ResourcePattern string    `yaml:"resource_pattern" json:"resource_pattern"`
	FieldPattern    string    `yaml:"field_pattern" json:"field_pattern"`
	Reason          string    `yaml:"reason" json:"reason"`
	ExpiresAt       time.Time `yaml:"expires_at" json:"expires_at"`
}

func ParseIgnoreFile(data []byte) (IgnoreFile, error) {
	var out IgnoreFile
	if len(data) == 0 {
		return out, nil
	}
	if err := yaml.Unmarshal(data, &out); err != nil {
		return out, err
	}
	for _, s := range out.Suppressions {
		if s.ExpiresAt.IsZero() {
			return out, ErrSuppressionMissingExpiration
		}
		if s.ResourcePattern == "" || s.FieldPattern == "" || s.Reason == "" {
			return out, errors.New("suppression_missing_required_field")
		}
	}
	return out, nil
}

func (f IgnoreFile) Suppresses(change PlanChange, now time.Time) bool {
	for _, s := range f.Suppressions {
		if now.After(s.ExpiresAt) {
			continue
		}
		resourceOK, _ := filepath.Match(s.ResourcePattern, change.Resource)
		fieldOK, _ := filepath.Match(s.FieldPattern, change.Field)
		if resourceOK && fieldOK {
			return true
		}
	}
	return false
}
