"""propose-cost-reduction skill (Phase 6).

The skill turns a Recommendation into a concrete PR draft. The PR body explains
the change, expected savings, and gates that must remain green (Phase 2 + Phase
4). Production wires this to GitHub MCP; the in-memory variant below is used
for tests.
"""

from __future__ import annotations

from abc import ABC, abstractmethod

from .models import Recommendation, RecommendationKind


class GitHubPRClient(ABC):
    @abstractmethod
    def open_pr(
        self,
        *,
        repo: str,
        title: str,
        body: str,
        branch: str,
        base: str = "main",
    ) -> str: ...


class StubGitHubPR(GitHubPRClient):
    def __init__(self) -> None:
        self.opened: list[dict] = []

    def open_pr(
        self,
        *,
        repo: str,
        title: str,
        body: str,
        branch: str,
        base: str = "main",
    ) -> str:
        url = f"https://github.com/{repo}/pull/{len(self.opened) + 1}"
        self.opened.append(
            {"repo": repo, "title": title, "body": body, "branch": branch, "base": base, "url": url}
        )
        return url


def render_pr_body(rec: Recommendation) -> str:
    return (
        f"## Cost-reduction proposal\n\n"
        f"**Recommendation:** {rec.title}\n\n"
        f"### Why\n{rec.detail}\n\n"
        f"### Expected savings\n"
        f"- **${rec.expected_savings_usd_monthly:.2f}/month** "
        f"(severity: {rec.severity})\n\n"
        f"### Affected resources\n"
        + "\n".join(f"- `{r}`" for r in rec.affected_resources)
        + "\n\n"
        f"### Required gates\n"
        f"- forge/lint, forge/test-with-coverage, forge/sast, forge/sca\n"
        f"- forge/sbom, forge/cosign-sign-attest, forge/openspec-link\n"
        f"- finops/savings-realised (Phase 4 gate)\n\n"
        f"_This PR was drafted by the autonomous FinOps advisor "
        f"(`source=autonomous-loop`)._\n"
    )


def repo_for(rec: Recommendation) -> str:
    """Best-effort repo inference. Production resolves via the registry asset."""
    if rec.kind == RecommendationKind.EXPENSIVE_LLM_SKILL:
        return "forge-skills"
    if rec.kind == RecommendationKind.CACHEABLE_PROMPT:
        return "forge-prompts"
    return "forge-iac-modules"


def branch_for(rec: Recommendation) -> str:
    return f"finops/{rec.kind.value}/{rec.id}"


def open_pr_for(rec: Recommendation, client: GitHubPRClient) -> str:
    title = f"finops: {rec.title}"
    body = render_pr_body(rec)
    repo = repo_for(rec)
    branch = branch_for(rec)
    url = client.open_pr(repo=repo, title=title, body=body, branch=branch)
    rec.pr_url = url
    rec.pr_status = "open"
    return url
