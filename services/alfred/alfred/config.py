"""Settings for Alfred service. All values come from env, with safe local defaults."""

from __future__ import annotations

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="", extra="ignore")

    # Server
    addr: str = Field(default="0.0.0.0:8090", alias="ADDR")
    log_level: str = Field(default="INFO", alias="LOG_LEVEL")

    # Auth
    keycloak_issuer: str = Field(default="http://localhost:8080/realms/forge", alias="KEYCLOAK_ISSUER")
    keycloak_audience: str = Field(default="forge-alfred", alias="KEYCLOAK_AUDIENCE")
    openfga_url: str = Field(default="http://localhost:8088", alias="OPENFGA_API_URL")
    openfga_store: str = Field(default="", alias="OPENFGA_STORE_ID")
    openfga_model: str = Field(default="", alias="OPENFGA_AUTHORIZATION_MODEL_ID")

    # LLM (always via LiteLLM — never direct providers)
    litellm_url: str = Field(default="http://localhost:4000", alias="LITELLM_URL")
    litellm_key: str = Field(default="sk-forge-local", alias="LITELLM_KEY")
    default_model: str = Field(default="gemini-1.5-pro", alias="ALFRED_DEFAULT_MODEL")

    # Internal services
    control_plane_url: str = Field(default="http://localhost:8081", alias="CONTROL_PLANE_URL")
    registry_url: str = Field(default="http://localhost:8082", alias="REGISTRY_URL")
    openspec_url: str = Field(default="http://localhost:8083", alias="OPENSPEC_URL")
    policy_engine_url: str = Field(default="http://localhost:8084", alias="POLICY_ENGINE_URL")
    approvals_url: str = Field(default="http://localhost:8105", alias="APPROVALS_URL")
    rag_query_url: str = Field(default="http://localhost:8086", alias="RAG_QUERY_URL")
    prompt_registry_url: str = Field(default="http://localhost:8087", alias="PROMPT_REGISTRY_URL")
    # alfred-litellm-header-injection (G2): the canonical prompt service.
    # Alfred's ToolRouter dispatches `prompt:<id>:render` to
    # `<prompt_template_service_url>/v1/render`. No default — refuse to start
    # without it so we never silently route to a stale or unconfigured target.
    prompt_template_service_url: str = Field(default="", alias="PROMPT_TEMPLATE_SERVICE_URL")
    permissions_url: str = Field(default="http://localhost:8092", alias="PERMISSIONS_URL")
    skill_runner_url: str = Field(default="http://localhost:8091", alias="SKILL_RUNNER_URL")
    mcp_github_url: str = Field(default="http://localhost:8101", alias="MCP_GITHUB_URL")
    mcp_jira_url: str = Field(default="http://localhost:8102", alias="MCP_JIRA_URL")
    mcp_confluence_url: str = Field(default="http://localhost:8103", alias="MCP_CONFLUENCE_URL")
    mcp_openspec_url: str = Field(default="http://localhost:8104", alias="MCP_OPENSPEC_URL")

    # Persistence
    postgres_url: str = Field(
        default="postgres://forge:forge@localhost:15432/forge_alfred?sslmode=disable",
        alias="POSTGRES_URL",
    )

    # Telemetry
    otlp_endpoint: str = Field(default="http://localhost:4318", alias="OTEL_EXPORTER_OTLP_ENDPOINT")
    langfuse_host: str = Field(default="http://localhost:3030", alias="LANGFUSE_HOST")
    langfuse_public_key: str = Field(default="", alias="LANGFUSE_PUBLIC_KEY")
    langfuse_secret_key: str = Field(default="", alias="LANGFUSE_SECRET_KEY")

    # Limits
    max_loop_iterations: int = Field(default=8, alias="ALFRED_MAX_LOOP")
    rag_top_k: int = Field(default=8, alias="ALFRED_RAG_TOPK")

    # Wizard / dialogue API (platform-gaps-closure 3.x). `disabled` by default;
    # flip to `enabled` to surface the /v1/intent/* routes.
    alfred_dialogue_api: str = Field(default="disabled", alias="ALFRED_DIALOGUE_API")

    # Agent-mode (alfred-agent-mode-orchestrator). Defaults off; flip to enable
    # the /v1/agent-mode/* routes. Workspace-level dock_enabled flag is checked
    # separately at request time.
    alfred_agent_mode_enabled: bool = Field(default=False, alias="ALFRED_AGENT_MODE_ENABLED")
    workflow_runtime_url: str = Field(default="http://localhost:8093", alias="WORKFLOW_RUNTIME_URL")
    agent_mode_preset_dir: str = Field(
        default="/var/lib/forge/alfred/presets", alias="ALFRED_PRESET_DIR"
    )
    agent_mode_default_model: str = Field(
        default="gemini-1.5-pro", alias="ALFRED_AGENT_MODE_MODEL"
    )

    # Alfred Console v2 (alfred-console-redesign). Per-tenant flag default=false.
    # Flip ALFRED_CONSOLE_V2_ENABLED=true to enable the Friendly/Advanced views.
    alfred_console_v2_enabled: bool = Field(default=False, alias="ALFRED_CONSOLE_V2_ENABLED")

    # Per-tenant LLM budget caps.
    # ALFRED_DEFAULT_LLM_BUDGET_USD — default USD budget per workspace per period.
    #   Override per-tenant via LiteLLM's /budget/new endpoint using
    #   workspace_id as the team_id; this config is the fallback when no
    #   tenant-specific budget row exists in LiteLLM.
    # ALFRED_BUDGET_WINDOW_HOURS — rolling window for the budget evaluation.
    #   Set to 24 for daily budgets, 168 for weekly.
    alfred_default_llm_budget_usd: float = Field(default=10.0, alias="ALFRED_DEFAULT_LLM_BUDGET_USD")
    alfred_budget_window_hours: int = Field(default=24, alias="ALFRED_BUDGET_WINDOW_HOURS")

    # Spec dedup threshold. Configurable per tenant; hard floor enforced at write.
    spec_match_threshold_default: float = Field(default=0.80, alias="SPEC_MATCH_THRESHOLD_DEFAULT")
    spec_match_threshold_floor: float = Field(default=0.65, alias="SPEC_MATCH_THRESHOLD_FLOOR")

    # Milvus / dedup index endpoint (reuses RAG infra).
    dedup_index_url: str = Field(default="http://localhost:8086", alias="DEDUP_INDEX_URL")


def load_settings() -> Settings:
    settings = Settings()
    if not settings.prompt_template_service_url:
        raise RuntimeError(
            "PROMPT_TEMPLATE_SERVICE_URL is required: Alfred dispatches "
            "prompt:<id>:render tools through prompt-template-service. "
            "Set the env var to the base URL (e.g. http://localhost:8099)."
        )
    return settings
