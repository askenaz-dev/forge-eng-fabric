package observability

import (
	"sort"
)

// AgentModeSessionMetrics is the workspace-level rollup of Alfred agent-mode
// sessions over the configured retention window of the store. It powers the
// session-level dashboard tile (cost p95, success rate, HITL-pause rate).
type AgentModeSessionMetrics struct {
	WorkspaceID         string  `json:"workspace_id"`
	Sessions            int     `json:"sessions"`
	Successes           int     `json:"successes"`
	SuccessRate         float64 `json:"success_rate"`
	HITLPaused          int     `json:"hitl_paused"`
	HITLPauseRate       float64 `json:"hitl_pause_rate"`
	CostUSDP50          float64 `json:"cost_usd_p50"`
	CostUSDP95          float64 `json:"cost_usd_p95"`
}

// AgentModeMetrics computes the rollup from the in-memory invocation store.
// Sessions are identified by AssetID prefix `alfred:agent-mode:`.
func (s *Service) AgentModeMetrics(workspaceID string) AgentModeSessionMetrics {
	out := AgentModeSessionMetrics{WorkspaceID: workspaceID}
	bySession := map[string]*Invocation{}
	hitlBySession := map[string]bool{}
	for _, inv := range s.Store.All() {
		if inv.Source != "alfred.agent_mode" {
			continue
		}
		if workspaceID != "" && inv.WorkspaceID != workspaceID {
			continue
		}
		sid := inv.AssetID
		acc, ok := bySession[sid]
		if !ok {
			cp := inv
			bySession[sid] = &cp
		} else {
			acc.LLMCostUSD += inv.LLMCostUSD
			if inv.Success {
				acc.Success = true
			}
		}
		// Mark HITL-paused if any invocation for the session signals a pause.
		// The event source emits a separate Invocation per state transition.
		if inv.DurationMS == 0 && !inv.Success {
			hitlBySession[sid] = true
		}
	}
	costs := make([]float64, 0, len(bySession))
	for sid, acc := range bySession {
		out.Sessions++
		if acc.Success {
			out.Successes++
		}
		if hitlBySession[sid] {
			out.HITLPaused++
		}
		costs = append(costs, acc.LLMCostUSD)
	}
	if out.Sessions > 0 {
		out.SuccessRate = float64(out.Successes) / float64(out.Sessions)
		out.HITLPauseRate = float64(out.HITLPaused) / float64(out.Sessions)
	}
	if len(costs) > 0 {
		sort.Float64s(costs)
		out.CostUSDP50 = percentile(costs, 0.50)
		out.CostUSDP95 = percentile(costs, 0.95)
	}
	return out
}
