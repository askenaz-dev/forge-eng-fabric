"""Unit tests for alfred-litellm-header-injection (G1).

Covers RequestContext validation, header injection on chat() and embed(),
fail-closed behavior on missing fields, and classification defaulting.
"""

from __future__ import annotations

from typing import Any

import httpx
import pytest

from alfred.llm import LiteLLMClient, LiteLLMHeaderError, RequestContext


def _ctx(**overrides: Any) -> RequestContext:
    base = dict(
        tenant_id="tenant-1",
        workspace_id="ws-1",
        correlation_id="corr-1",
    )
    base.update(overrides)
    return RequestContext(**base)


def test_request_context_headers_populated() -> None:
    headers = _ctx().headers()
    assert headers == {
        "forgetenantid": "tenant-1",
        "forgeworkspaceid": "ws-1",
        "forgecorrelationid": "corr-1",
        "data_classification": "internal",
    }


def test_request_context_default_classification_is_internal() -> None:
    assert _ctx().data_classification == "internal"
    assert _ctx().headers()["data_classification"] == "internal"


def test_request_context_explicit_classification_overrides_default() -> None:
    ctx = _ctx(data_classification="confidential")
    assert ctx.headers()["data_classification"] == "confidential"


@pytest.mark.parametrize("field", ["tenant_id", "workspace_id", "correlation_id"])
def test_request_context_missing_required_field_raises(field: str) -> None:
    with pytest.raises(LiteLLMHeaderError, match=field):
        _ctx(**{field: ""}).headers()


def test_request_context_unknown_classification_raises() -> None:
    with pytest.raises(LiteLLMHeaderError, match="data_classification"):
        _ctx(data_classification="top_secret").headers()


def test_request_context_system_factory_populates_all_fields() -> None:
    ctx = RequestContext.system(correlation_id="corr-sys")
    assert ctx.tenant_id == "system"
    assert ctx.workspace_id == "system"
    assert ctx.correlation_id == "corr-sys"
    assert ctx.data_classification == "internal"
    assert ctx.headers()["forgetenantid"] == "system"


class _CapturingTransport(httpx.AsyncBaseTransport):
    """Captures the latest outbound request so tests can assert headers."""

    def __init__(self, payload: dict[str, Any]) -> None:
        self.payload = payload
        self.last_request: httpx.Request | None = None

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        self.last_request = request
        return httpx.Response(200, json=self.payload, request=request)


@pytest.fixture
def fake_transport(monkeypatch: pytest.MonkeyPatch) -> _CapturingTransport:
    transport = _CapturingTransport(
        payload={
            "choices": [{"message": {"content": "ok"}}],
            "data": [{"embedding": [0.1, 0.2]}],
        }
    )
    original = httpx.AsyncClient.__init__

    def _patched_init(self: httpx.AsyncClient, *args: Any, **kwargs: Any) -> None:
        kwargs.setdefault("transport", transport)
        original(self, *args, **kwargs)

    monkeypatch.setattr(httpx.AsyncClient, "__init__", _patched_init)
    return transport


async def test_chat_carries_all_four_headers(fake_transport: _CapturingTransport) -> None:
    client = LiteLLMClient("http://litellm.test", "sk-key")
    await client.chat(
        model="gpt-4o-mini",
        messages=[{"role": "user", "content": "hi"}],
        context=_ctx(),
    )
    assert fake_transport.last_request is not None
    headers = fake_transport.last_request.headers
    assert headers["forgetenantid"] == "tenant-1"
    assert headers["forgeworkspaceid"] == "ws-1"
    assert headers["forgecorrelationid"] == "corr-1"
    assert headers["data_classification"] == "internal"
    assert headers["authorization"] == "Bearer sk-key"


async def test_embed_carries_all_four_headers(fake_transport: _CapturingTransport) -> None:
    client = LiteLLMClient("http://litellm.test", "sk-key")
    await client.embed(
        model="text-embedding-3-small",
        inputs=["hello"],
        context=_ctx(data_classification="confidential"),
    )
    assert fake_transport.last_request is not None
    headers = fake_transport.last_request.headers
    assert headers["forgetenantid"] == "tenant-1"
    assert headers["forgeworkspaceid"] == "ws-1"
    assert headers["forgecorrelationid"] == "corr-1"
    assert headers["data_classification"] == "confidential"


async def test_chat_with_missing_tenant_id_fails_closed(
    fake_transport: _CapturingTransport,
) -> None:
    client = LiteLLMClient("http://litellm.test", "sk-key")
    with pytest.raises(LiteLLMHeaderError):
        await client.chat(
            model="gpt-4o-mini",
            messages=[{"role": "user", "content": "hi"}],
            context=_ctx(tenant_id=""),
        )
    # No outbound call MUST have been issued.
    assert fake_transport.last_request is None


async def test_embed_with_missing_correlation_id_fails_closed(
    fake_transport: _CapturingTransport,
) -> None:
    client = LiteLLMClient("http://litellm.test", "sk-key")
    with pytest.raises(LiteLLMHeaderError):
        await client.embed(
            model="text-embedding-3-small",
            inputs=["hello"],
            context=_ctx(correlation_id=""),
        )
    assert fake_transport.last_request is None
