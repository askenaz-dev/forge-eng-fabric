package fingerprint

import (
	"fmt"
	"sort"
	"strings"
)

// knownDimensions is the full set of allowed fingerprint dimensions.
var knownDimensions = map[string]bool{
	"service":     true,
	"signal":      true,
	"tenant":      true,
	"workspace":   true,
	"error_class": true,
	"port":        true,
	"route":       true,
}

// Dims is an ordered map of fingerprint dimensions.
type Dims map[string]string

// Build produces the canonical fingerprint string from a set of dimensions.
// Required: "service" and "signal". Returns an error for unknown or empty required dims.
func Build(dims Dims) (string, error) {
	if dims["service"] == "" {
		return "", fmt.Errorf("fingerprint: required dimension 'service' is missing or empty")
	}
	if dims["signal"] == "" {
		return "", fmt.Errorf("fingerprint: required dimension 'signal' is missing or empty")
	}

	pairs := make([]string, 0, len(dims))
	for k, v := range dims {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if !knownDimensions[k] {
			return "", fmt.Errorf("fingerprint: unknown dimension %q", k)
		}
		pairs = append(pairs, k+":"+v)
	}

	sort.Strings(pairs)
	return strings.Join(pairs, "|"), nil
}

// Validate checks that fp is a valid canonical fingerprint: all dimensions known,
// sorted, required dims present, no duplicates.
func Validate(fp string) error {
	if fp == "" {
		return fmt.Errorf("fingerprint: empty")
	}
	parts := strings.Split(fp, "|")
	seen := map[string]bool{}
	prev := ""
	hasService, hasSignal := false, false
	for _, p := range parts {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return fmt.Errorf("fingerprint: malformed pair %q", p)
		}
		k := kv[0]
		if !knownDimensions[k] {
			return fmt.Errorf("fingerprint: unknown dimension %q", k)
		}
		if seen[k] {
			return fmt.Errorf("fingerprint: duplicate dimension %q", k)
		}
		seen[k] = true
		if prev != "" && k < prev {
			return fmt.Errorf("fingerprint: dimensions not sorted (got %q after %q)", k, prev)
		}
		prev = k
		if k == "service" {
			hasService = true
		}
		if k == "signal" {
			hasSignal = true
		}
	}
	if !hasService {
		return fmt.Errorf("fingerprint: missing required dimension 'service'")
	}
	if !hasSignal {
		return fmt.Errorf("fingerprint: missing required dimension 'signal'")
	}
	return nil
}
