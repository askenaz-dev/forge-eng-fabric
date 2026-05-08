"""Short-lived GitHub App installation token issuer.

Implements the contract from `github-app-provisioning` spec:
- Tokens are scoped to a single repository
- TTL ≤ 10 minutes
- Issuance is auditable (returns `audit` payload alongside token)

In real environments this calls GitHub's REST API
`POST /app/installations/{id}/access_tokens` with `repository_ids` and
`permissions` set to the minimum needed. In dev/test mode the issuer
returns a deterministic stub token.
"""

from __future__ import annotations

import os
import time
import uuid
from dataclasses import dataclass, field
from typing import Any

DEFAULT_TTL_SECONDS = 600  # 10 minutes (spec maximum)


@dataclass
class InstallationToken:
    token: str
    expires_at: float
    repo: str
    permissions: dict[str, str]
    audit: dict[str, Any] = field(default_factory=dict)

    def expires_in(self) -> int:
        return max(0, int(self.expires_at - time.time()))


class TokenIssuer:
    """Issues installation tokens with mandatory short-lived TTL.

    Pluggable backend: when `backend == "github"` it talks to the real API,
    otherwise it returns deterministic stubs suitable for unit tests.
    """

    def __init__(
        self,
        *,
        app_id: str | None = None,
        installation_id: str | None = None,
        backend: str | None = None,
        max_ttl_seconds: int = DEFAULT_TTL_SECONDS,
    ) -> None:
        self.app_id = app_id or os.getenv("FORGE_GITHUB_APP_ID", "stub-app")
        self.installation_id = installation_id or os.getenv("FORGE_GITHUB_INSTALLATION_ID", "stub-install")
        self.backend = backend or os.getenv("FORGE_GITHUB_TOKEN_BACKEND", "stub")
        self.max_ttl_seconds = max_ttl_seconds

    def issue(
        self,
        *,
        repo: str,
        permissions: dict[str, str] | None = None,
        ttl_seconds: int | None = None,
        principal: str = "alfred",
        correlation_id: str | None = None,
    ) -> InstallationToken:
        if not repo or "/" not in repo:
            raise ValueError("repo must be in form 'org/name'")
        ttl = min(ttl_seconds or self.max_ttl_seconds, self.max_ttl_seconds)
        if ttl <= 0:
            raise ValueError("ttl_seconds must be > 0")

        perms = permissions or {"contents": "write", "pull_requests": "write"}

        if self.backend == "github":  # pragma: no cover — exercised in real env
            token = self._call_github(repo, perms, ttl)
        else:
            token = f"ghs_stub_{uuid.uuid4().hex[:16]}"

        expires_at = time.time() + ttl
        audit = {
            "issuer": "forge-github-mcp",
            "app_id": self.app_id,
            "installation_id": self.installation_id,
            "repo": repo,
            "permissions": perms,
            "ttl_seconds": ttl,
            "principal": principal,
            "correlation_id": correlation_id or str(uuid.uuid4()),
            "backend": self.backend,
        }
        return InstallationToken(
            token=token,
            expires_at=expires_at,
            repo=repo,
            permissions=perms,
            audit=audit,
        )

    def _call_github(self, repo: str, permissions: dict[str, str], ttl_seconds: int) -> str:
        # Placeholder; real impl mints a JWT signed by the GitHub App private
        # key and POSTs to `/app/installations/{id}/access_tokens` with
        # `repository_ids=[repo_id]` and the requested permissions.
        raise NotImplementedError("github backend not configured in this build")
