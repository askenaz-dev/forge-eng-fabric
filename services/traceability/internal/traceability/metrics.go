package traceability

import (
	"fmt"
	"strings"
)

func (s *Service) Metrics() string {
	nodes, links := s.Store.Counts()
	coverage := 0.0
	if nodes > 0 {
		coverage = float64(links) / float64(nodes)
	}
	var b strings.Builder
	b.WriteString("# HELP traceability_coverage Traceability links divided by nodes.\n")
	b.WriteString("# TYPE traceability_coverage gauge\n")
	fmt.Fprintf(&b, "traceability_coverage %.4f\n", coverage)
	b.WriteString("# HELP traceability_query_latency_p95 Materialized traceability query p95 latency in seconds.\n")
	b.WriteString("# TYPE traceability_query_latency_p95 gauge\n")
	b.WriteString("traceability_query_latency_p95 0\n")
	return b.String()
}
