package sdlc

import (
	"fmt"
	"strings"
)

func (s *Service) Metrics() string {
	initiatives := s.Store.List("")
	var gatesTotal, gatesPassed, blocked int
	var b strings.Builder
	b.WriteString("# HELP sdlc_phase_duration_seconds SDLC phase duration in seconds.\n")
	b.WriteString("# TYPE sdlc_phase_duration_seconds gauge\n")
	for _, initiative := range initiatives {
		for _, state := range initiative.PhaseStates {
			if state.EnteredAt != nil && state.CompletedAt != nil {
				fmt.Fprintf(&b, "sdlc_phase_duration_seconds{workspace=\"%s\",phase=\"%s\"} %.0f\n", initiative.WorkspaceID, state.Phase, state.CompletedAt.Sub(*state.EnteredAt).Seconds())
			}
			for _, gate := range state.Gates {
				gatesTotal++
				if gate.Outcome == GatePassed {
					gatesPassed++
				}
			}
			if len(state.Blockers) > 0 || state.Status == StatusBlocked {
				blocked++
			}
		}
	}
	b.WriteString("# HELP gate_pass_rate Passed SDLC gates divided by evaluated gates.\n")
	b.WriteString("# TYPE gate_pass_rate gauge\n")
	fmt.Fprintf(&b, "gate_pass_rate %.4f\n", ratio(gatesPassed, gatesTotal))
	b.WriteString("# HELP phase_block_rate Blocked phase states divided by initiatives.\n")
	b.WriteString("# TYPE phase_block_rate gauge\n")
	fmt.Fprintf(&b, "phase_block_rate %.4f\n", ratio(blocked, len(initiatives)))
	return b.String()
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
