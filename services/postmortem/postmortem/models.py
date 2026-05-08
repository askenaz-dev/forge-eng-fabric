"""Models for postmortem generation."""

from __future__ import annotations

from datetime import datetime, timezone

from pydantic import BaseModel, Field


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


class TimelineEvent(BaseModel):
    occurred_at: datetime
    label: str
    detail: str | None = None
    source: str | None = None


class HealingActionRecord(BaseModel):
    action_id: str
    level: str
    outcome: str
    workflow_run_id: str | None = None


class ActionItem(BaseModel):
    title: str
    owner: str
    due_date: datetime | None = None
    severity: str = "medium"
    description: str | None = None
    jira_issue_key: str | None = None


class PostmortemRequest(BaseModel):
    incident_id: str
    tenant_id: str
    workspace_id: str | None = None
    asset_id: str | None = None
    service: str
    environment: str
    severity: str
    summary: str
    impact: str
    timeline: list[TimelineEvent] = Field(default_factory=list)
    healing_actions: list[HealingActionRecord] = Field(default_factory=list)
    diagnosis_summary: str | None = None
    diagnosis_citations: list[dict] = Field(default_factory=list)
    started_at: datetime | None = None
    resolved_at: datetime | None = None
    synthetic: bool = False


class PostmortemDraft(BaseModel):
    incident_id: str
    title: str
    body_markdown: str
    action_items: list[ActionItem]
    sections: list[str]
    citations: list[dict]
    generated_at: datetime = Field(default_factory=utcnow)


class PublishResult(BaseModel):
    incident_id: str
    confluence_url: str
    openspec_link: str
    jira_issue_keys: list[str]
    published_at: datetime = Field(default_factory=utcnow)
