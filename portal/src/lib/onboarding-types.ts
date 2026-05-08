export type TemplateParameter = {
  type: string;
  description?: string;
  required?: boolean;
  default?: unknown;
  pattern?: string;
  enum?: string[];
};

export type RepoTemplate = {
  id: string;
  version: string;
  description?: string;
  category?: string;
  lifecycle_state: string;
  trust_level: string;
  parameters?: Record<string, TemplateParameter>;
  required_capabilities?: string[];
};

export type OnboardingRequest = {
  id: string;
  workspace_id: string;
  tenant_id: string;
  repo_org: string;
  repo_name: string;
  template_id: string;
  template_version: string;
  parameters?: Record<string, unknown>;
  criticality: string;
  data_classification: string;
  owners: string[];
  status: string;
  status_reason?: string;
  asset_id?: string;
  correlation_id: string;
  requested_by: string;
  created_at: string;
  completed_at?: string;
};

export type OnboardingEvent = {
  id: string;
  request_id: string;
  stage: string;
  outcome: string;
  message?: string;
  payload?: Record<string, unknown>;
  duration_ms?: number;
  created_at: string;
};

export type PipelineGateResult = {
  workspace_id: string;
  repo_full_name: string;
  pr_number?: number;
  commit_sha: string;
  stage: string;
  tool: string;
  outcome: string;
  severity_counts?: Record<string, unknown>;
  report_url?: string;
  policy_version?: string;
  created_at: string;
};
