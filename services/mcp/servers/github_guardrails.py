"""Guardrails for GitHub MCP write tools.

Enforces (per `mcp-and-skills` spec delta):
- Allowlist of GitHub orgs (per Workspace).
- Workspace scope: deny mutations targeting an org bound to a different Workspace.
- Schema validation for mutation parameters.
- Deny destructive operations (`delete_repo`, `force_push`) without explicit override.

Designed to run as a sync hook before tool execution. Returns a tuple of
`(allowed: bool, rationale: str, audit: dict)` so the MCP can audit denials
and short-circuit to HTTP 403.
"""

from __future__ import annotations

import os
import re
from dataclasses import dataclass
from typing import Any

REPO_NAME_RE = re.compile(r"^[A-Za-z0-9._-]{1,100}$")
BRANCH_NAME_RE = re.compile(r"^[A-Za-z0-9._/-]{1,255}$")
DESTRUCTIVE_TOOLS = {"mcp:github.delete_repo", "mcp:github.force_push"}

# Default in-memory mapping; production deployments populate from
# control-plane (workspace -> orgs). Wide open in dev unless env restricts.
DEFAULT_WORKSPACE_ORG_MAP: dict[str, set[str]] = {}


@dataclass
class GuardrailDecision:
    allowed: bool
    rationale: str
    audit: dict[str, Any]


class GuardrailEngine:
    def __init__(
        self,
        *,
        workspace_org_map: dict[str, set[str]] | None = None,
        org_allowlist: set[str] | None = None,
    ) -> None:
        self.workspace_org_map = workspace_org_map or dict(DEFAULT_WORKSPACE_ORG_MAP)
        env_allow = os.getenv("FORGE_GITHUB_ORG_ALLOWLIST", "")
        env_set = {o.strip() for o in env_allow.split(",") if o.strip()}
        self.org_allowlist = org_allowlist or env_set or None

    def check(
        self,
        *,
        tool_id: str,
        params: dict[str, Any],
        workspace_id: str | None,
        principal: str,
        correlation_id: str | None,
        approved_overrides: set[str] | None = None,
    ) -> GuardrailDecision:
        approved_overrides = approved_overrides or set()
        audit_base: dict[str, Any] = {
            "tool_id": tool_id,
            "principal": principal,
            "workspace_id": workspace_id,
            "correlation_id": correlation_id,
        }

        repo = str(params.get("repo") or "")
        org = repo.split("/", 1)[0] if "/" in repo else ""
        repo_name = repo.split("/", 1)[1] if "/" in repo else ""

        if tool_id in DESTRUCTIVE_TOOLS and "allow-force-push" not in approved_overrides and "delete-repo" not in approved_overrides:
            return GuardrailDecision(
                False,
                "destructive_op_denied",
                {**audit_base, "guardrail": "destructive_op", "reason": "no override"},
            )

        if not org or not repo_name:
            return GuardrailDecision(False, "invalid_repo_name", {**audit_base, "guardrail": "schema"})

        if not REPO_NAME_RE.match(repo_name):
            return GuardrailDecision(False, "invalid_repo_name", {**audit_base, "guardrail": "schema"})

        branch = params.get("branch")
        if branch is not None and not BRANCH_NAME_RE.match(str(branch)):
            return GuardrailDecision(False, "invalid_branch_name", {**audit_base, "guardrail": "schema"})

        if self.org_allowlist is not None and org not in self.org_allowlist:
            return GuardrailDecision(
                False,
                "org_not_allowlisted",
                {**audit_base, "guardrail": "org_allowlist", "org": org},
            )

        if workspace_id and workspace_id in self.workspace_org_map:
            allowed_orgs = self.workspace_org_map[workspace_id]
            if org not in allowed_orgs:
                return GuardrailDecision(
                    False,
                    "cross_workspace_denied",
                    {**audit_base, "guardrail": "workspace_scope", "org": org, "allowed": sorted(allowed_orgs)},
                )

        return GuardrailDecision(
            True,
            "allowed",
            {**audit_base, "guardrail": "passed", "org": org},
        )
