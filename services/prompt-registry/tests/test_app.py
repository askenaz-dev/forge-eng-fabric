from __future__ import annotations

from fastapi.testclient import TestClient

from prompt_registry.app import create_app

TEMPLATE = {
    "id": "summarize-openspec",
    "version": "1.0.0",
    "owner_team": "sdlc",
    "template": "Summarize {{ title }} for {{ audience }}",
    "variables_schema": {
        "type": "object",
        "required": ["title", "audience"],
        "properties": {"title": {"type": "string"}, "audience": {"type": "string"}},
    },
    "examples": [{"title": "Payments", "audience": "SRE"}],
    "recommended_model": "gemini-1.5-pro",
    "guardrails": {"max_tokens": 20},
    "trust_level": "T1",
}


def test_prompt_registry_render_and_approval_threshold() -> None:
    app = create_app()
    with TestClient(app) as client:
        created = client.post("/v1/templates", json=TEMPLATE)
        assert created.status_code == 201
        assert created.json()["lifecycle_state"] == "proposed"

        invalid = client.post("/v1/templates/summarize-openspec/render", json={"variables": {"title": "Payments"}})
        assert invalid.status_code == 400

        rendered = client.post(
            "/v1/templates/summarize-openspec/render",
            json={"variables": {"title": "Payments", "audience": "SRE"}},
        )
        assert rendered.status_code == 200
        assert rendered.json()["rendered"] == "Summarize Payments for SRE"

        blocked = client.post(
            "/v1/templates/summarize-openspec/versions/1.0.0/promote",
            json={"lifecycle_state": "approved", "eval_scores": {"quality": 0.7}, "actor": "owner"},
        )
        assert blocked.status_code == 400

        approved = client.post(
            "/v1/templates/summarize-openspec/versions/1.0.0/promote",
            json={"lifecycle_state": "approved", "eval_scores": {"quality": 0.9}, "actor": "owner"},
        )
        assert approved.status_code == 200
        assert approved.json()["lifecycle_state"] == "approved"
