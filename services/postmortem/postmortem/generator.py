"""Postmortem generation pipeline.

Triggered by `incident.resolved.v1`. The generator:
  1. assembles timeline + healing-action records + diagnosis citations.
  2. runs a versioned prompt against an LLM (stubbed in tests).
  3. evaluates the draft against the postmortem eval suite.
  4. publishes to Confluence, files Jira issues for action items, and links
     the postmortem from the OpenSpec of the affected asset.

The LLM client is abstracted; the default StubLLM produces a deterministic
draft suitable for tests and synthetic incident flows.
"""

from __future__ import annotations

from abc import ABC, abstractmethod
from datetime import datetime, timezone
from typing import Any

from .events import Sink, new_event
from .models import (
    ActionItem,
    HealingActionRecord,
    PostmortemDraft,
    PostmortemRequest,
    PublishResult,
    TimelineEvent,
)
from .prompts import GENERATE_POSTMORTEM_PROMPT, GENERATE_POSTMORTEM_PROMPT_VERSION
from .publishers import (
    ConfluencePublisher,
    JiraIssueCreator,
    OpenSpecLinker,
    StubConfluence,
    StubJira,
    StubOpenSpec,
    publish_all,
)


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


REQUIRED_SECTIONS = (
    "## Summary",
    "## Impact",
    "## Timeline",
    "## Root cause",
    "## What went well",
    "## What went wrong",
    "## Remediation",
    "## Lessons",
    "## Action items",
)


class LLMClient(ABC):
    @abstractmethod
    def complete(self, *, prompt: str, context: dict[str, Any]) -> dict[str, Any]: ...


class StubLLM(LLMClient):
    """Deterministic stub used for tests + synthetic flows."""

    def complete(self, *, prompt: str, context: dict[str, Any]) -> dict[str, Any]:
        req = context["request"]
        action_items = [
            {
                "title": f"Track follow-up on {action['action_id']}",
                "owner": "@sre-platform",
                "severity": "medium",
            }
            for action in context.get("healing_actions", [])
        ] or [
            {
                "title": "Investigate root cause exhaustively",
                "owner": "@sre-platform",
                "severity": "medium",
            }
        ]
        return {
            "title": f"Postmortem — {req['service']} {req['environment']} {req['summary']}",
            "summary": req["summary"],
            "impact": req["impact"] or "no measurable customer impact recorded",
            "timeline": "\n".join(
                f"- {ev['occurred_at']} — {ev['label']}" for ev in context.get("timeline", [])
            )
            or "- (no events recorded)",
            "what_went_well": "- detection signal fired correctly\n- diagnosis emitted citations",
            "what_went_wrong": f"- {req['service']} reached degraded state",
            "root_cause": req.get("diagnosis_summary") or "Root cause requires further analysis",
            "remediation": "\n".join(
                f"- {a['action_id']} → {a['outcome']}" for a in context.get("healing_actions", [])
            )
            or "- (none applied automatically)",
            "lessons": "- runbook entry should be linked from the alert",
            "action_items": action_items,
        }


class PostmortemGenerator:
    """High-level orchestrator."""

    def __init__(
        self,
        sink: Sink,
        *,
        llm: LLMClient | None = None,
        confluence: ConfluencePublisher | None = None,
        jira: JiraIssueCreator | None = None,
        openspec: OpenSpecLinker | None = None,
    ) -> None:
        self.sink = sink
        self.llm = llm or StubLLM()
        self.confluence = confluence or StubConfluence()
        self.jira = jira or StubJira()
        self.openspec = openspec or StubOpenSpec()
        self._prompt_version = GENERATE_POSTMORTEM_PROMPT_VERSION

    def generate(self, req: PostmortemRequest) -> PostmortemDraft:
        raw = self.llm.complete(
            prompt=GENERATE_POSTMORTEM_PROMPT,
            context=self._build_context(req),
        )
        draft = self._render_draft(req, raw)
        self.sink.emit(
            new_event(
                tenant_id=req.tenant_id,
                workspace_id=req.workspace_id,
                event_type="postmortem.generated.v1",
                subject=f"incident/{req.incident_id}",
                data={
                    "incident_id": req.incident_id,
                    "prompt_version": self._prompt_version,
                    "section_count": len(draft.sections),
                    "synthetic": req.synthetic,
                },
            )
        )
        return draft

    def publish(self, draft: PostmortemDraft, req: PostmortemRequest) -> PublishResult:
        confluence_url, openspec_link, issue_keys = publish_all(
            draft,
            confluence=self.confluence,
            jira=self.jira,
            openspec=self.openspec,
            space=req.tenant_id,
            jira_project="FORGE",
            asset_id=req.asset_id,
        )
        result = PublishResult(
            incident_id=req.incident_id,
            confluence_url=confluence_url,
            openspec_link=openspec_link,
            jira_issue_keys=issue_keys,
        )
        self.sink.emit(
            new_event(
                tenant_id=req.tenant_id,
                workspace_id=req.workspace_id,
                event_type="postmortem.published.v1",
                subject=f"incident/{req.incident_id}",
                data={
                    "incident_id": req.incident_id,
                    "confluence_url": confluence_url,
                    "openspec_link": openspec_link,
                    "jira_issues": issue_keys,
                    "synthetic": req.synthetic,
                },
            )
        )
        return result

    def evaluate(self, draft: PostmortemDraft, req: PostmortemRequest) -> dict[str, Any]:
        """Run the eval suite over a draft.

        Returns `{passed: bool, failures: [...]}`. The eval suite checks:
          - each REQUIRED_SECTION appears in the body.
          - every diagnosis citation source_id appears in the body.
          - every action item has a non-empty owner.
        """
        failures: list[str] = []
        for section in REQUIRED_SECTIONS:
            if section not in draft.body_markdown:
                failures.append(f"missing_section: {section}")
        for citation in req.diagnosis_citations:
            sid = citation.get("source_id", "")
            if sid and sid not in draft.body_markdown:
                failures.append(f"missing_citation: {sid}")
        for item in draft.action_items:
            if not item.owner.strip() or item.owner.strip() == "@unknown":
                failures.append(f"action_item_missing_owner: {item.title}")
        return {"passed": not failures, "failures": failures}

    # --- helpers ---

    def _build_context(self, req: PostmortemRequest) -> dict[str, Any]:
        return {
            "request": req.model_dump(mode="json"),
            "timeline": [t.model_dump(mode="json") for t in req.timeline],
            "healing_actions": [a.model_dump(mode="json") for a in req.healing_actions],
            "diagnosis_summary": req.diagnosis_summary,
            "diagnosis_citations": req.diagnosis_citations,
        }

    def _render_draft(self, req: PostmortemRequest, raw: dict[str, Any]) -> PostmortemDraft:
        action_items = [
            ActionItem(
                title=item.get("title", ""),
                owner=item.get("owner", "@unknown"),
                severity=item.get("severity", "medium"),
                description=item.get("description"),
            )
            for item in raw.get("action_items", [])
        ]
        body = self._render_body(req, raw, action_items)
        return PostmortemDraft(
            incident_id=req.incident_id,
            title=raw.get("title", f"Postmortem — {req.service}"),
            body_markdown=body,
            action_items=action_items,
            sections=list(REQUIRED_SECTIONS),
            citations=req.diagnosis_citations,
        )

    @staticmethod
    def _render_body(
        req: PostmortemRequest,
        raw: dict[str, Any],
        action_items: list[ActionItem],
    ) -> str:
        lines: list[str] = []
        lines.append(f"# {raw.get('title', 'Postmortem')}")
        lines.append("")
        lines.append("## Summary")
        lines.append(raw.get("summary", req.summary))
        lines.append("")
        lines.append("## Impact")
        lines.append(raw.get("impact", req.impact))
        lines.append("")
        lines.append("## Timeline")
        lines.append(raw.get("timeline", _format_timeline(req.timeline)))
        lines.append("")
        lines.append("## Root cause")
        lines.append(raw.get("root_cause", req.diagnosis_summary or ""))
        lines.append("")
        lines.append("## What went well")
        lines.append(raw.get("what_went_well", "- (n/a)"))
        lines.append("")
        lines.append("## What went wrong")
        lines.append(raw.get("what_went_wrong", "- (n/a)"))
        lines.append("")
        lines.append("## Remediation")
        lines.append(raw.get("remediation", _format_actions(req.healing_actions)))
        lines.append("")
        lines.append("## Lessons")
        lines.append(raw.get("lessons", "- (n/a)"))
        lines.append("")
        lines.append("## Action items")
        for item in action_items:
            lines.append(
                f"- [ ] {item.title} (owner: {item.owner}, severity: {item.severity})"
            )
        # Diagnosis citations are deliberately NOT auto-appended here. The eval
        # suite expects the prose (root cause / remediation / lessons) to cite
        # evidence by source_id; an auto-footer would mask drafts that fail to
        # do so.
        return "\n".join(lines)


def _format_timeline(timeline: list[TimelineEvent]) -> str:
    if not timeline:
        return "- (no events recorded)"
    return "\n".join(f"- {ev.occurred_at.isoformat()} — {ev.label}" for ev in timeline)


def _format_actions(actions: list[HealingActionRecord]) -> str:
    if not actions:
        return "- (none applied automatically)"
    return "\n".join(f"- {a.action_id} ({a.level}) → {a.outcome}" for a in actions)
