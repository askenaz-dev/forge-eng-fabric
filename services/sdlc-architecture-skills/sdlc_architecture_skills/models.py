"""Pydantic request/response models for SDLC architecture skills."""

from __future__ import annotations

from pydantic import BaseModel, Field


# ---------------------------------------------------------------------------
# propose-adr
# ---------------------------------------------------------------------------


class ProposeAdrRequest(BaseModel):
    tenant_id: str
    workspace_id: str | None = None
    title: str
    context: str
    decision_drivers: list[str] = Field(default_factory=list)
    options_considered: list[str] = Field(default_factory=list)
    output_dir: str = "docs/adr"


class ProposeAdrResponse(BaseModel):
    status: str  # "ok" | "error"
    adr_path: str
    adr_number: int
    slug: str
    event_type: str = "sdlc.adr.proposed.v1"


# ---------------------------------------------------------------------------
# evaluate-options
# ---------------------------------------------------------------------------


class EvaluateOption(BaseModel):
    name: str
    description: str | None = None


class EvaluateOptionsRequest(BaseModel):
    tenant_id: str
    workspace_id: str | None = None
    decision_context: str
    options: list[EvaluateOption]
    criteria: list[str] = Field(default_factory=list)


class RankedOption(BaseModel):
    name: str
    score: float = Field(ge=0.0, le=1.0)
    pros: list[str]
    cons: list[str]
    rationale: str


class EvaluateOptionsResponse(BaseModel):
    status: str
    ranked_options: list[RankedOption]


# ---------------------------------------------------------------------------
# check-openspec-alignment
# ---------------------------------------------------------------------------


class CheckAlignmentRequest(BaseModel):
    tenant_id: str
    workspace_id: str | None = None
    spec_path: str
    requirements: list[str]


class CheckAlignmentResponse(BaseModel):
    status: str
    aligned: bool
    unaddressed_requirements: list[str]
    notes: str | None = None


# ---------------------------------------------------------------------------
# generate-api-contract
# ---------------------------------------------------------------------------


class GenerateApiContractRequest(BaseModel):
    tenant_id: str
    workspace_id: str | None = None
    service_name: str
    endpoints: list[dict]
    output_dir: str = "contracts/openapi"


class GenerateApiContractResponse(BaseModel):
    status: str
    openapi_path: str
    spectral_lint_passed: bool
    lint_warnings: list[str] = Field(default_factory=list)


# ---------------------------------------------------------------------------
# propose-data-model
# ---------------------------------------------------------------------------


class ProposeDataModelRequest(BaseModel):
    tenant_id: str
    workspace_id: str | None = None
    domain: str
    entities: list[str]
    relationships: list[str] = Field(default_factory=list)
    output_dir: str = "docs/data-model"


class ProposeDataModelResponse(BaseModel):
    status: str
    model_path: str
    event_type: str = "sdlc.data_model.proposed.v1"


# ---------------------------------------------------------------------------
# lightweight-threat-model
# ---------------------------------------------------------------------------


class LightweightThreatModelRequest(BaseModel):
    tenant_id: str
    workspace_id: str | None = None
    system_name: str
    trust_boundaries: list[str] = Field(default_factory=list)
    data_flows: list[str] = Field(default_factory=list)
    output_dir: str = "docs/threat-models"


class ThreatFinding(BaseModel):
    category: str  # STRIDE category
    description: str
    severity: str  # low | medium | high | critical
    mitigation: str


class LightweightThreatModelResponse(BaseModel):
    status: str
    threat_model_path: str
    findings: list[ThreatFinding]
    event_type: str = "sdlc.threat_model.completed.v1"
