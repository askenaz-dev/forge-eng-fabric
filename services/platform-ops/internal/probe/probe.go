// Package probe implements pre-validate and post-validate probes for platform-ops endpoints.
package probe

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ExpectedOutcome describes what a successful action looks like.
type ExpectedOutcome struct {
	Kind    string `json:"kind"`    // "http_healthz" | "tcp_connect" | "none"
	Target  string `json:"target"`  // URL or addr
	Timeout string `json:"timeout"` // e.g. "10s"
}

// Verify checks that the expected outcome is met.
// Returns nil on success, an error with a description on failure.
func Verify(ctx context.Context, o ExpectedOutcome) error {
	if o.Kind == "none" || o.Kind == "" {
		return nil
	}

	timeout := 10 * time.Second
	if o.Timeout != "" {
		if d, err := time.ParseDuration(o.Timeout); err == nil {
			timeout = d
		}
	}

	switch o.Kind {
	case "http_healthz":
		return httpCheck(ctx, o.Target, timeout)
	default:
		return fmt.Errorf("probe: unknown kind %q", o.Kind)
	}
}

func httpCheck(ctx context.Context, url string, timeout time.Duration) error {
	c := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("probe: build request: %w", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("probe: request failed: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("probe: service unhealthy (HTTP %d)", resp.StatusCode)
	}
	return nil
}
