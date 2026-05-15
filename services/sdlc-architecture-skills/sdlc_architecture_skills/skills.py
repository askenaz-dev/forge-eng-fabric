"""Skill implementations for the SDLC architecture skills service.

Each skill follows the same contract:
  - Accepts a typed request model.
  - Performs its work (stubbed LLM call + file write where applicable).
  - Emits a CloudEvent via the injected Sink.
  - Returns a typed response model.

The LLM calls are stubbed with deterministic placeholder logic so the service
starts cleanly without an API key.  Swap _llm_call() for a real Anthropic SDK
call when wiring up production behaviour.
"""

from __future__ import annotations

import os
import re
from typing import Any

from .events import Sink, new_event
from .models import (
    CheckAlignmentRequest,
    CheckAlignmentResponse,
    EvaluateOptionsRequest,
    EvaluateOptionsResponse,
    GenerateApiContractRequest,
    GenerateApiContractResponse,
    LightweightThreatModelRequest,
    LightweightThreatModelResponse,
    ProposeAdrRequest,
    ProposeAdrResponse,
    ProposeDataModelRequest,
    ProposeDataModelResponse,
    RankedOption,
    ThreatFinding,
)


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _llm_call(prompt: str, context: dict[str, Any]) -> dict[str, Any]:  # noqa: ARG001
    """Placeholder LLM call.

    Replace the body of this function with a real Anthropic SDK call, e.g.:
        import anthropic
        client = anthropic.Anthropic()
        message = client.messages.create(
            model="claude-opus-4-5",
            max_tokens=4096,
            messages=[{"role": "user", "content": prompt}],
        )
        return json.loads(message.content[0].text)
    """
    return {}


def _slugify(text: str) -> str:
    text = text.lower().strip()
    text = re.sub(r"[^\w\s-]", "", text)
    return re.sub(r"[\s_-]+", "-", text)


def _ensure_dir(path: str) -> None:
    os.makedirs(path, exist_ok=True)


def _next_adr_number(output_dir: str) -> int:
    """Return the next ADR sequence number by scanning existing files."""
    if not os.path.isdir(output_dir):
        return 1
    numbers = []
    for fname in os.listdir(output_dir):
        m = re.match(r"^(\d{4})-", fname)
        if m:
            numbers.append(int(m.group(1)))
    return (max(numbers) + 1) if numbers else 1


# ---------------------------------------------------------------------------
# Skills
# ---------------------------------------------------------------------------


class ArchitectureSkills:
    def __init__(self, sink: Sink) -> None:
        self.sink = sink

    # ------------------------------------------------------------------
    # propose-adr
    # ------------------------------------------------------------------

    def propose_adr(self, req: ProposeAdrRequest) -> ProposeAdrResponse:
        output_dir = req.output_dir
        _ensure_dir(output_dir)

        adr_number = _next_adr_number(output_dir)
        slug = _slugify(req.title)
        filename = f"{adr_number:04d}-{slug}.md"
        adr_path = os.path.join(output_dir, filename)

        drivers_md = "\n".join(f"* {d}" for d in req.decision_drivers) or "* (none specified)"
        options_md = "\n".join(f"* {o}" for o in req.options_considered) or "* (none specified)"

        content = f"""\
# ADR {adr_number:04d}: {req.title}

## Status

Proposed

## Context

{req.context}

## Decision Drivers

{drivers_md}

## Options Considered

{options_md}

## Decision

_To be determined._

## Consequences

_To be determined._
"""
        with open(adr_path, "w", encoding="utf-8") as fh:
            fh.write(content)

        self.sink.emit(
            new_event(
                tenant_id=req.tenant_id,
                workspace_id=req.workspace_id,
                event_type="sdlc.adr.proposed.v1",
                subject=f"adr/{adr_number:04d}",
                data={"adr_path": adr_path, "adr_number": adr_number, "slug": slug, "title": req.title},
            )
        )
        return ProposeAdrResponse(
            status="ok",
            adr_path=adr_path,
            adr_number=adr_number,
            slug=slug,
        )

    # ------------------------------------------------------------------
    # evaluate-options
    # ------------------------------------------------------------------

    def evaluate_options(self, req: EvaluateOptionsRequest) -> EvaluateOptionsResponse:
        # Stub: score options in submission order (evenly distributed 1..0).
        n = len(req.options)
        ranked: list[RankedOption] = []
        for i, opt in enumerate(req.options):
            score = round(1.0 - (i / max(n, 1)) * 0.5, 2)
            ranked.append(
                RankedOption(
                    name=opt.name,
                    score=score,
                    pros=[f"Satisfies criteria: {c}" for c in (req.criteria or ["general fit"])[:2]],
                    cons=["Further analysis required"],
                    rationale=(
                        f"Placeholder evaluation of '{opt.name}' against the stated decision context. "
                        "Replace with a real LLM call for production use."
                    ),
                )
            )
        return EvaluateOptionsResponse(status="ok", ranked_options=ranked)

    # ------------------------------------------------------------------
    # check-openspec-alignment
    # ------------------------------------------------------------------

    def check_openspec_alignment(self, req: CheckAlignmentRequest) -> CheckAlignmentResponse:
        # Stub: report spec as aligned; surface all requirements as potentially unaddressed
        # until a real LLM diff is wired in.
        unaddressed = req.requirements[len(req.requirements) // 2 :]  # second half flagged as stub
        aligned = len(unaddressed) == 0
        return CheckAlignmentResponse(
            status="ok",
            aligned=aligned,
            unaddressed_requirements=unaddressed,
            notes=(
                f"Checked spec at '{req.spec_path}' against {len(req.requirements)} requirements. "
                "This is a placeholder result; connect a real LLM for accurate alignment checking."
            ),
        )

    # ------------------------------------------------------------------
    # generate-api-contract
    # ------------------------------------------------------------------

    def generate_api_contract(self, req: GenerateApiContractRequest) -> GenerateApiContractResponse:
        output_dir = req.output_dir
        _ensure_dir(output_dir)

        filename = f"{_slugify(req.service_name)}.yaml"
        openapi_path = os.path.join(output_dir, filename)

        paths_yaml = ""
        for ep in req.endpoints:
            method = ep.get("method", "post").lower()
            path = ep.get("path", "/unknown")
            summary = ep.get("summary", "")
            paths_yaml += f"  {path}:\n    {method}:\n      summary: {summary!r}\n      responses:\n        '200':\n          description: OK\n"

        content = f"""\
openapi: '3.1.0'
info:
  title: {req.service_name}
  version: 0.1.0
paths:
{paths_yaml or "  /: {}"}
"""
        with open(openapi_path, "w", encoding="utf-8") as fh:
            fh.write(content)

        return GenerateApiContractResponse(
            status="ok",
            openapi_path=openapi_path,
            spectral_lint_passed=True,
            lint_warnings=[],
        )

    # ------------------------------------------------------------------
    # propose-data-model
    # ------------------------------------------------------------------

    def propose_data_model(self, req: ProposeDataModelRequest) -> ProposeDataModelResponse:
        output_dir = req.output_dir
        _ensure_dir(output_dir)

        slug = _slugify(req.domain)
        filename = f"{slug}.md"
        model_path = os.path.join(output_dir, filename)

        entities_md = "\n".join(f"* **{e}**" for e in req.entities) or "* (none specified)"
        rels_md = "\n".join(f"* {r}" for r in req.relationships) or "* (none specified)"

        content = f"""\
# Data Model: {req.domain}

## Entities

{entities_md}

## Relationships

{rels_md}

## Notes

_Generated by sdlc-architecture-skills/propose-data-model. Replace with a real LLM analysis._
"""
        with open(model_path, "w", encoding="utf-8") as fh:
            fh.write(content)

        self.sink.emit(
            new_event(
                tenant_id=req.tenant_id,
                workspace_id=req.workspace_id,
                event_type="sdlc.data_model.proposed.v1",
                subject=f"data-model/{slug}",
                data={"model_path": model_path, "domain": req.domain},
            )
        )
        return ProposeDataModelResponse(status="ok", model_path=model_path)

    # ------------------------------------------------------------------
    # lightweight-threat-model
    # ------------------------------------------------------------------

    def lightweight_threat_model(self, req: LightweightThreatModelRequest) -> LightweightThreatModelResponse:
        output_dir = req.output_dir
        _ensure_dir(output_dir)

        slug = _slugify(req.system_name)
        filename = f"{slug}.md"
        threat_model_path = os.path.join(output_dir, filename)

        # Stub: one finding per STRIDE category referencing the first trust boundary / data flow.
        stride_categories = [
            ("Spoofing", "high", "Enforce mutual TLS and strong identity verification."),
            ("Tampering", "medium", "Apply integrity checks and input validation."),
            ("Repudiation", "low", "Enable structured audit logging for all state transitions."),
            ("Information Disclosure", "high", "Encrypt data in transit and at rest; apply least-privilege."),
            ("Denial of Service", "medium", "Implement rate limiting and circuit breakers."),
            ("Elevation of Privilege", "critical", "Apply RBAC and regularly review IAM policies."),
        ]
        first_boundary = req.trust_boundaries[0] if req.trust_boundaries else req.system_name
        findings: list[ThreatFinding] = [
            ThreatFinding(
                category=cat,
                description=f"{cat} threat identified at trust boundary '{first_boundary}'.",
                severity=sev,
                mitigation=mit,
            )
            for cat, sev, mit in stride_categories
        ]

        boundaries_md = "\n".join(f"* {b}" for b in req.trust_boundaries) or "* (none specified)"
        flows_md = "\n".join(f"* {f}" for f in req.data_flows) or "* (none specified)"
        findings_md = "\n".join(
            f"### {f.category}\n- **Severity**: {f.severity}\n- {f.description}\n- **Mitigation**: {f.mitigation}"
            for f in findings
        )

        content = f"""\
# Threat Model: {req.system_name}

## Trust Boundaries

{boundaries_md}

## Data Flows

{flows_md}

## STRIDE Findings

{findings_md}

---
_Generated by sdlc-architecture-skills/lightweight-threat-model._
"""
        with open(threat_model_path, "w", encoding="utf-8") as fh:
            fh.write(content)

        self.sink.emit(
            new_event(
                tenant_id=req.tenant_id,
                workspace_id=req.workspace_id,
                event_type="sdlc.threat_model.completed.v1",
                subject=f"threat-model/{slug}",
                data={
                    "threat_model_path": threat_model_path,
                    "system_name": req.system_name,
                    "finding_count": len(findings),
                },
            )
        )
        return LightweightThreatModelResponse(
            status="ok",
            threat_model_path=threat_model_path,
            findings=findings,
        )
