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
    approvals_url: str = Field(default="http://localhost:8085", alias="APPROVALS_URL")
    rag_query_url: str = Field(default="http://localhost:8086", alias="RAG_QUERY_URL")
    prompt_registry_url: str = Field(default="http://localhost:8087", alias="PROMPT_REGISTRY_URL")
    permissions_url: str = Field(default="http://localhost:8092", alias="PERMISSIONS_URL")
    skill_runner_url: str = Field(default="http://localhost:8091", alias="SKILL_RUNNER_URL")
    mcp_github_url: str = Field(default="http://localhost:8101", alias="MCP_GITHUB_URL")
    mcp_jira_url: str = Field(default="http://localhost:8102", alias="MCP_JIRA_URL")
    mcp_confluence_url: str = Field(default="http://localhost:8103", alias="MCP_CONFLUENCE_URL")
    mcp_openspec_url: str = Field(default="http://localhost:8104", alias="MCP_OPENSPEC_URL")

    # Persistence
    postgres_url: str = Field(
        default="postgres://forge:forge@localhost:5432/forge_alfred?sslmode=disable",
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


def load_settings() -> Settings:
    return Settings()
