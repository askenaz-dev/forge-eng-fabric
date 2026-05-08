"""External system adapters for postmortem publishing.

Production wires these to the Confluence MCP, Jira MCP, and OpenSpec service
respectively. The in-memory variants below are used in tests and synthetic
flows so the rest of the pipeline can be exercised without external deps.
"""

from __future__ import annotations

from abc import ABC, abstractmethod

from .models import ActionItem, PostmortemDraft


class ConfluencePublisher(ABC):
    @abstractmethod
    def publish(self, *, space: str, title: str, body_markdown: str) -> str: ...


class JiraIssueCreator(ABC):
    @abstractmethod
    def create(self, *, project: str, item: ActionItem, postmortem_url: str) -> str: ...


class OpenSpecLinker(ABC):
    @abstractmethod
    def link(self, *, asset_id: str | None, postmortem_url: str) -> str: ...


class StubConfluence(ConfluencePublisher):
    def __init__(self) -> None:
        self.published: list[dict] = []

    def publish(self, *, space: str, title: str, body_markdown: str) -> str:
        url = f"confluence://{space}/pm-{len(self.published) + 1}"
        self.published.append({"space": space, "title": title, "body": body_markdown, "url": url})
        return url


class StubJira(JiraIssueCreator):
    def __init__(self) -> None:
        self.created: list[dict] = []

    def create(self, *, project: str, item: ActionItem, postmortem_url: str) -> str:
        key = f"{project}-{len(self.created) + 1}"
        self.created.append({"key": key, "title": item.title, "owner": item.owner, "url": postmortem_url})
        return key


class StubOpenSpec(OpenSpecLinker):
    def __init__(self) -> None:
        self.links: list[dict] = []

    def link(self, *, asset_id: str | None, postmortem_url: str) -> str:
        link = f"openspec://link/{asset_id or 'global'}/postmortem"
        self.links.append({"asset_id": asset_id, "postmortem_url": postmortem_url, "link": link})
        return link


def publish_all(
    draft: PostmortemDraft,
    *,
    confluence: ConfluencePublisher,
    jira: JiraIssueCreator,
    openspec: OpenSpecLinker,
    space: str = "FORGE",
    jira_project: str = "FORGE",
    asset_id: str | None = None,
) -> tuple[str, str, list[str]]:
    """Best-effort fan-out to all external systems."""
    confluence_url = confluence.publish(space=space, title=draft.title, body_markdown=draft.body_markdown)
    issue_keys: list[str] = []
    for item in draft.action_items:
        key = jira.create(project=jira_project, item=item, postmortem_url=confluence_url)
        item.jira_issue_key = key
        issue_keys.append(key)
    openspec_link = openspec.link(asset_id=asset_id, postmortem_url=confluence_url)
    return confluence_url, openspec_link, issue_keys
