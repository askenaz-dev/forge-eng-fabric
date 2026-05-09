"""Question-generation for the Alfred wizard.

Drives the conversational intent-capture flow: given the current draft state,
returns the next question to ask the user. The implementation walks a cached
list of domain templates first (cheap, fast) and falls back to a LiteLLM call
only for novel branches — mitigating the "Wizard LLM cost" risk in the
platform-gaps-closure design.

When LiteLLM is unavailable, the function still returns a sensible question
from the templates so local development can exercise the full flow without an
LLM gateway.
"""

from __future__ import annotations

from typing import Any

from alfred.llm import LiteLLMClient

MAX_DIALOGUE_TURNS = 12

# Cached domain templates ordered by priority. The wizard asks the first
# question whose section is `partial` or `empty`. Each entry maps a section
# name to the prompt that surfaces when the section is incomplete.
SECTION_QUESTIONS: dict[str, dict[str, str]] = {
    "intent": {
        "title": "What's a short title for this initiative?",
        "business_intent": "In one paragraph, what business outcome are you trying to achieve?",
        "problem_statement": "What problem does this solve, and for whom?",
    },
    "stakeholders": {
        "stakeholders": "Who are the stakeholders? (one role per line, e.g., Product, Engineering, Customer Support)",
        "success_metrics": "How will you measure success? List the metrics or signals you'll watch.",
    },
    "requirements": {
        "functional": "What are the must-have functional capabilities? (one per line)",
        "non_functional": "Any non-functional requirements? (latency, availability, compliance)",
        "constraints": "Any constraints, integrations, or things this must NOT do?",
    },
    "autonomy": {
        "default_mode": "How much autonomy should Alfred have for routine decisions: autonomous, requires_approval, or restricted?",
        "approvals_required": "Which actions should always require human approval? (production deploys, schema changes, etc.)",
    },
}


def next_question(completeness: dict[str, Any] | None, turn_count: int) -> tuple[str | None, str | None]:
    """Return (section, question) for the next turn, or (None, None) when no
    more questions are needed.

    `completeness` is the response shape from `GET /v1/openspecs/{id}/completeness`.
    Once `turn_count` exceeds MAX_DIALOGUE_TURNS, returns (None, None) so the
    caller can prompt the user to commit instead.
    """
    if turn_count >= MAX_DIALOGUE_TURNS:
        return None, None
    if not completeness:
        return "intent", SECTION_QUESTIONS["intent"]["business_intent"]
    for section_status in completeness.get("sections", []):
        section_name = section_status.get("name")
        section_state = section_status.get("status")
        if section_state == "complete":
            continue
        questions = SECTION_QUESTIONS.get(section_name, {})
        for field_name, field_state in section_status.get("fields", {}).items():
            if field_state in {"empty", "partial"} and field_name in questions:
                return section_name, questions[field_name]
    return None, None


async def generate_followup(
    *,
    completeness: dict[str, Any] | None,
    turn_count: int,
    last_answer: str,
    correlation_id: str | None,
    llm: LiteLLMClient | None,
) -> tuple[str | None, str | None]:
    """Wrapper around `next_question` that calls LiteLLM only when the cached
    template path is exhausted. The LLM path is intentionally a thin override:
    if the gateway is unavailable, we return the cached question.

    Returns (section, question).
    """
    section, question = next_question(completeness, turn_count)
    if question is not None or llm is None:
        return section, question

    # Cached templates exhausted — ask the LLM for a closing/clarifying question.
    # Production wiring would inject `last_answer` and the partial OpenSpec.
    try:
        response = await llm.chat(
            model="gpt-4o-mini",
            messages=[
                {
                    "role": "system",
                    "content": (
                        "You are Alfred, the Forge Control Plane Agent. Continue a wizard-style dialogue "
                        "to capture an OpenSpec. The user has already answered structured questions. "
                        "Ask one follow-up clarifying question, no more than two sentences. Return only "
                        "the question text; no preamble."
                    ),
                },
                {"role": "user", "content": last_answer or ""},
            ],
            metadata={"correlation_id": correlation_id, "purpose": "wizard_followup"},
            max_tokens=120,
        )
        choices = response.get("choices") or []
        if not choices:
            return None, None
        text = (choices[0].get("message") or {}).get("content")
        if not text:
            return None, None
        return None, text.strip()
    except Exception:
        # Caller falls back to "ready to commit" prompt.
        return None, None
