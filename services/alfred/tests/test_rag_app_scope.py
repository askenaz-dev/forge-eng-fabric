"""Tests for app-first-class-entity 7.3: App-scoped RAG retrieval.

The RAG retriever scopes queries to the App's corpus by default, with a
workspace fallback when the App corpus is empty. Every retrieval logs its
effective scope so observability can spot the fallback frequency.
"""

from __future__ import annotations

import importlib.util
import uuid

import pytest


def _load_gateways():
    spec = importlib.util.spec_from_file_location(
        "alfred_gateways",
        "alfred/gateways.py",
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class FakeHttpClient:
    def __init__(self, responses: list[dict[str, list[dict]]]):
        self._responses = responses
        self.calls: list[dict] = []

    async def __aenter__(self):
        return self

    async def __aexit__(self, *args):
        return None

    async def post(self, url, json):
        self.calls.append({"url": url, "json": json})
        # Pop the queued response.
        index = min(len(self.calls) - 1, len(self._responses) - 1)
        payload = self._responses[index]

        class Response:
            status_code = 200

            def json(self):
                return payload

        return Response()


@pytest.mark.asyncio
async def test_query_with_app_scope_uses_app_corpus_first(monkeypatch):
    gw = _load_gateways()
    client = gw.RAGClient("http://rag")
    fake = FakeHttpClient([{"results": [{"id": "doc-from-app"}]}])
    monkeypatch.setattr(gw.httpx, "AsyncClient", lambda timeout=None: fake)
    observed = []
    ws_id = uuid.uuid4()
    app_id = uuid.uuid4()
    hits = await client.query_with_app_scope(
        workspace_id=ws_id,
        text="hello",
        app_id=app_id,
        observe=lambda log: observed.append(log),
    )
    assert hits == [{"id": "doc-from-app"}]
    assert fake.calls[0]["json"]["app_id"] == str(app_id)
    assert observed[0]["scope"] == "app"


@pytest.mark.asyncio
async def test_query_with_app_scope_falls_back_to_workspace_when_app_empty(monkeypatch):
    gw = _load_gateways()
    client = gw.RAGClient("http://rag")
    fake = FakeHttpClient([
        {"results": []},               # App scope returns nothing
        {"results": [{"id": "doc-from-ws"}]},  # Workspace fallback returns one
    ])
    monkeypatch.setattr(gw.httpx, "AsyncClient", lambda timeout=None: fake)
    observed = []
    ws_id = uuid.uuid4()
    app_id = uuid.uuid4()
    hits = await client.query_with_app_scope(
        workspace_id=ws_id,
        text="hello",
        app_id=app_id,
        observe=lambda log: observed.append(log),
    )
    assert hits == [{"id": "doc-from-ws"}]
    # First call carries app_id, second call drops it.
    assert fake.calls[0]["json"].get("app_id") == str(app_id)
    assert "app_id" not in fake.calls[1]["json"]
    assert observed[0]["scope"] == "workspace_fallback"


@pytest.mark.asyncio
async def test_query_with_app_scope_workspace_only_when_no_app_id(monkeypatch):
    gw = _load_gateways()
    client = gw.RAGClient("http://rag")
    fake = FakeHttpClient([{"results": [{"id": "doc-ws"}]}])
    monkeypatch.setattr(gw.httpx, "AsyncClient", lambda timeout=None: fake)
    observed = []
    hits = await client.query_with_app_scope(
        workspace_id=uuid.uuid4(),
        text="hello",
        app_id=None,
        observe=lambda log: observed.append(log),
    )
    assert hits == [{"id": "doc-ws"}]
    assert "app_id" not in fake.calls[0]["json"]
    assert observed[0]["scope"] == "workspace"
