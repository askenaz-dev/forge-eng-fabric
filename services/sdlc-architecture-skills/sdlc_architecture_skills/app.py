"""FastAPI app factory for the SDLC architecture skills service."""

from __future__ import annotations

from fastapi import FastAPI, HTTPException

from .events import LogSink, Sink
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
)
from .skills import ArchitectureSkills


def create_app(sink: Sink | None = None) -> FastAPI:
    _sink = sink or LogSink()
    skills = ArchitectureSkills(sink=_sink)
    app = FastAPI(title="sdlc-architecture-skills", version="0.1.0")
    app.state.skills = skills

    @app.get("/healthz")
    def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/skills/propose-adr", response_model=ProposeAdrResponse)
    def propose_adr(req: ProposeAdrRequest) -> ProposeAdrResponse:
        s: ArchitectureSkills = app.state.skills
        try:
            return s.propose_adr(req)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    @app.post("/v1/skills/evaluate-options", response_model=EvaluateOptionsResponse)
    def evaluate_options(req: EvaluateOptionsRequest) -> EvaluateOptionsResponse:
        s: ArchitectureSkills = app.state.skills
        try:
            return s.evaluate_options(req)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    @app.post("/v1/skills/check-openspec-alignment", response_model=CheckAlignmentResponse)
    def check_openspec_alignment(req: CheckAlignmentRequest) -> CheckAlignmentResponse:
        s: ArchitectureSkills = app.state.skills
        try:
            return s.check_openspec_alignment(req)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    @app.post("/v1/skills/generate-api-contract", response_model=GenerateApiContractResponse)
    def generate_api_contract(req: GenerateApiContractRequest) -> GenerateApiContractResponse:
        s: ArchitectureSkills = app.state.skills
        try:
            return s.generate_api_contract(req)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    @app.post("/v1/skills/propose-data-model", response_model=ProposeDataModelResponse)
    def propose_data_model(req: ProposeDataModelRequest) -> ProposeDataModelResponse:
        s: ArchitectureSkills = app.state.skills
        try:
            return s.propose_data_model(req)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    @app.post("/v1/skills/lightweight-threat-model", response_model=LightweightThreatModelResponse)
    def lightweight_threat_model(req: LightweightThreatModelRequest) -> LightweightThreatModelResponse:
        s: ArchitectureSkills = app.state.skills
        try:
            return s.lightweight_threat_model(req)
        except Exception as exc:
            raise HTTPException(status_code=500, detail=str(exc)) from exc

    return app


app = create_app()
