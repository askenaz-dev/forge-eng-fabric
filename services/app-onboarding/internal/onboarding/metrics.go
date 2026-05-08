package onboarding

import (
	"fmt"
	"strings"
)

func (s *Store) PrometheusMetrics() string {
	s.mu.RLock()
	requests := make([]Request, 0, len(s.requests))
	for _, request := range s.requests {
		requests = append(requests, *request)
	}
	gates := append([]PipelineGateResult(nil), s.gateResults...)
	links := append([]PROpenSpecLink(nil), s.prLinks...)
	signatures := append([]ImageSignature(nil), s.imageSignatures...)
	overrideCount := s.overrideCount
	s.mu.RUnlock()

	var b strings.Builder
	statusCounts := map[Status]int{}
	completed := 0
	failed := 0
	durationCount := 0
	durationSum := 0.0
	buckets := []float64{60, 120, 300, 600}
	bucketCounts := make([]int, len(buckets)+1)

	for _, request := range requests {
		statusCounts[request.Status]++
		if request.Status == StatusCompleted {
			completed++
		}
		if request.Status == StatusFailed {
			failed++
		}
		if request.CompletedAt == nil || request.CreatedAt.IsZero() {
			continue
		}
		duration := request.CompletedAt.Sub(request.CreatedAt).Seconds()
		if duration < 0 {
			duration = 0
		}
		durationCount++
		durationSum += duration
		for i, bucket := range buckets {
			if duration <= bucket {
				bucketCounts[i]++
			}
		}
		bucketCounts[len(bucketCounts)-1]++
	}

	b.WriteString("# HELP onboarding_requests_total App onboarding requests by terminal or live status.\n")
	b.WriteString("# TYPE onboarding_requests_total counter\n")
	for _, status := range []Status{StatusPending, StatusPendingApproval, StatusRunning, StatusCompleted, StatusFailed} {
		fmt.Fprintf(&b, "onboarding_requests_total{status=%q} %d\n", status, statusCounts[status])
	}

	b.WriteString("# HELP onboarding_duration_seconds App onboarding completion duration.\n")
	b.WriteString("# TYPE onboarding_duration_seconds histogram\n")
	for i, bucket := range buckets {
		fmt.Fprintf(&b, "onboarding_duration_seconds_bucket{le=\"%.0f\"} %d\n", bucket, bucketCounts[i])
	}
	fmt.Fprintf(&b, "onboarding_duration_seconds_bucket{le=\"+Inf\"} %d\n", bucketCounts[len(bucketCounts)-1])
	fmt.Fprintf(&b, "onboarding_duration_seconds_sum %.6f\n", durationSum)
	fmt.Fprintf(&b, "onboarding_duration_seconds_count %d\n", durationCount)

	b.WriteString("# HELP onboarding_success_rate Successful app onboardings divided by completed plus failed onboardings.\n")
	b.WriteString("# TYPE onboarding_success_rate gauge\n")
	fmt.Fprintf(&b, "onboarding_success_rate %.6f\n", ratio(completed, completed+failed))

	gateFailures := 0
	for _, gate := range gates {
		if gate.Outcome == "fail" || gate.Outcome == "failed" {
			gateFailures++
		}
	}
	b.WriteString("# HELP pipeline_gate_failure_rate Failed pipeline gates divided by evaluated gates.\n")
	b.WriteString("# TYPE pipeline_gate_failure_rate gauge\n")
	fmt.Fprintf(&b, "pipeline_gate_failure_rate %.6f\n", ratio(gateFailures, len(gates)))

	linked := 0
	for _, link := range links {
		if link.Status == "linked" {
			linked++
		}
	}
	b.WriteString("# HELP pr_openspec_link_coverage PR OpenSpec links with status linked divided by tracked PR links.\n")
	b.WriteString("# TYPE pr_openspec_link_coverage gauge\n")
	fmt.Fprintf(&b, "pr_openspec_link_coverage %.6f\n", ratio(linked, len(links)))

	signed := 0
	for _, signature := range signatures {
		if signature.SignatureVerified && signature.AttestationVerified {
			signed++
		}
	}
	b.WriteString("# HELP image_signing_rate Verified image signatures and attestations divided by tracked images.\n")
	b.WriteString("# TYPE image_signing_rate gauge\n")
	fmt.Fprintf(&b, "image_signing_rate %.6f\n", ratio(signed, len(signatures)))

	b.WriteString("# HELP override_count Onboarding policy overrides granted.\n")
	b.WriteString("# TYPE override_count counter\n")
	fmt.Fprintf(&b, "override_count %d\n", overrideCount)
	return b.String()
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
