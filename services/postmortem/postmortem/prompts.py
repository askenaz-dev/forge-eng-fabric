"""Versioned prompt templates for postmortem generation."""

from __future__ import annotations

GENERATE_POSTMORTEM_PROMPT_VERSION = "generate-postmortem@1.0.0"

GENERATE_POSTMORTEM_PROMPT = """\
You are the Forge postmortem author. Write a structured postmortem for the
incident below. Output JSON with:

{
  "title": "<one-line title>",
  "summary": "<2-4 sentence summary>",
  "impact": "<scope + duration impact>",
  "timeline": "<markdown bullets>",
  "what_went_well": "<bullets>",
  "what_went_wrong": "<bullets>",
  "root_cause": "<root cause>",
  "remediation": "<bullets — what was done>",
  "lessons": "<bullets>",
  "action_items": [
    {"title": "...", "owner": "<owner-handle>", "severity": "low|medium|high"}
  ]
}

REQUIREMENTS (the eval suite enforces these):
- Every section MUST be present. Do not omit a heading even if the section is short.
- Each citation passed in evidence MUST appear at least once in the body.
- Every action_item MUST have an owner. Anonymous owners are rejected.
- Use the timeline events provided; do not invent timestamps.
- Use the healing_actions provided; do not invent action ids.
"""
