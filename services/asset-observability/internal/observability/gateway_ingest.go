package observability

import (
	"encoding/json"
	"time"
)

// IngestGatewayCloudEvent maps `com.forge.gateway.invocation.v1` and
// `com.forge.gateway.installed.v1` envelopes onto the in-memory store. The
// envelope shape mirrors what the skill-gateway producer emits.
func (s *Service) IngestGatewayCloudEvent(eventType string, body []byte) error {
	switch eventType {
	case "com.forge.gateway.invocation.v1":
		var ev struct {
			Data struct {
				Route        string  `json:"route"`
				AssetID      string  `json:"asset_id"`
				AssetVersion string  `json:"asset_version"`
				TenantID     string  `json:"tenant_id"`
				WorkspaceID  string  `json:"workspace_id"`
				DeveloperSub string  `json:"developer_sub"`
				StatusCode   int     `json:"status_code"`
				LatencyMS    float64 `json:"latency_ms"`
				CostUSDCents int64   `json:"cost_usd_cents"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &ev); err != nil {
			return err
		}
		inv := Invocation{
			AssetID:      ev.Data.AssetID,
			AssetVersion: ev.Data.AssetVersion,
			TenantID:     ev.Data.TenantID,
			WorkspaceID:  ev.Data.WorkspaceID,
			StartedAt:    time.Now().UTC(),
			DurationMS:   ev.Data.LatencyMS,
			Success:      ev.Data.StatusCode/100 == 2,
			LLMCostUSD:   float64(ev.Data.CostUSDCents) / 100.0,
			Source:       "gateway",
		}
		s.Store.Ingest(inv)
		return nil
	case "com.forge.gateway.installed.v1":
		var ev struct {
			Data struct {
				AssetID       string `json:"asset_id"`
				AssetVersion  string `json:"asset_version"`
				TenantID      string `json:"tenant_id"`
				DeveloperSub  string `json:"developer_sub"`
				Client        string `json:"client"`
				PackageDigest string `json:"package_digest"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &ev); err != nil {
			return err
		}
		s.Store.RecordInstall(Install{
			AssetID:       ev.Data.AssetID,
			AssetVersion:  ev.Data.AssetVersion,
			TenantID:      ev.Data.TenantID,
			DeveloperSub:  ev.Data.DeveloperSub,
			Client:        ev.Data.Client,
			PackageDigest: ev.Data.PackageDigest,
			InstalledAt:   time.Now().UTC(),
		})
		return nil
	case "alfred.agent_mode.session_started.v1",
		"alfred.agent_mode.step_started.v1",
		"alfred.agent_mode.step_completed.v1",
		"alfred.agent_mode.plan_revised.v1",
		"alfred.agent_mode.paused_for_approval.v1",
		"alfred.agent_mode.paused_for_budget.v1",
		"alfred.agent_mode.resumed.v1",
		"alfred.agent_mode.completed.v1",
		"alfred.agent_mode.aborted.v1",
		"alfred.agent_mode.failed.v1":
		return s.ingestAgentModeEvent(eventType, body)
	}
	return nil
}

// ingestAgentModeEvent rolls Alfred agent-mode events into the per-asset
// observability store so the workspace can see cost_per_session_p95,
// HITL-pause rate and success rate per session over time.
func (s *Service) ingestAgentModeEvent(eventType string, body []byte) error {
	var ev struct {
		Data struct {
			SessionID   string  `json:"session_id"`
			WorkspaceID string  `json:"workspace_id"`
			ModelID     string  `json:"model_id"`
			OpenSpecID  string  `json:"openspec_id"`
			StepIdx     int     `json:"step_idx"`
			Kind        string  `json:"kind"`
			CostUSD     float64 `json:"cost_usd"`
			LatencyMS   float64 `json:"latency_ms"`
			Reason      string  `json:"reason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		return err
	}
	inv := Invocation{
		AssetID:      "alfred:agent-mode:" + ev.Data.SessionID,
		AssetVersion: ev.Data.ModelID,
		WorkspaceID:  ev.Data.WorkspaceID,
		StartedAt:    time.Now().UTC(),
		DurationMS:   ev.Data.LatencyMS,
		Success:      eventType == "alfred.agent_mode.completed.v1",
		LLMCostUSD:   ev.Data.CostUSD,
		Source:       "alfred.agent_mode",
	}
	s.Store.Ingest(inv)
	return nil
}
