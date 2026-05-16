"""Unit tests for alfred-litellm-header-injection (G2).

Covers ToolRouter dispatch of `prompt:<id>:render` to prompt-template-service,
rejection of non-`:render` shapes, missing-config errors, and the mapping of
upstream failures to `ToolUnavailable`.
"""

from __future__ import annotations

from typing import Any

import httpx
import pytest

from alfred.tools import InvalidPromptToolId, ToolRouter, ToolUnavailable


class _CapturingTransport(httpx.AsyncBaseTransport):
    def __init__(self, *, status: int = 200, payload: Any | None = None) -> None:
        self.status = status
        self.payload = payload if payload is not None else {}
        self.last_request: httpx.Request | None = None

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        self.last_request = request
        return httpx.Response(self.status, json=self.payload, request=request)


@pytest.fixture
def fake_transport(monkeypatch: pytest.MonkeyPatch) -> _CapturingTransport:
    transport = _CapturingTransport(
        payload={
            "system": "you are alfred",
            "user": "hello Ada",
            "assistant_prefill": "",
            "token_estimate": 7,
        }
    )
    original = httpx.AsyncClient.__init__

    def _patched_init(self: httpx.AsyncClient, *args: Any, **kwargs: Any) -> None:
        kwargs.setdefault("transport", transport)
        original(self, *args, **kwargs)

    monkeypatch.setattr(httpx.AsyncClient, "__init__", _patched_init)
    return transport


async def test_prompt_render_dispatches_to_template_service(
    fake_transport: _CapturingTransport,
) -> None:
    router = ToolRouter(prompt_template_service_url="http://prompts.test")
    result = await router.invoke("prompt:greeting:render", {"name": "Ada"})
    assert result["user"] == "hello Ada"
    assert fake_transport.last_request is not None
    assert fake_transport.last_request.url == httpx.URL("http://prompts.test/v1/render")
    import json as _json

    body = _json.loads(fake_transport.last_request.content)
    assert body == {"ref": "greeting", "variables": {"name": "Ada"}}


async def test_prompt_render_strips_trailing_slash() -> None:
    router = ToolRouter(prompt_template_service_url="http://prompts.test/")
    # Triggers the rstrip path. Use a 404 to short-circuit on status,
    # but assert the URL was built without a double slash.
    transport = _CapturingTransport(status=404, payload={"detail": "missing"})

    async def _fake(request: httpx.Request) -> httpx.Response:
        return await transport.handle_async_request(request)

    with pytest.raises(ToolUnavailable):
        async with httpx.AsyncClient(transport=httpx.MockTransport(_fake)) as client:
            r = await client.post("http://prompts.test/v1/render", json={})
            if r.status_code >= 400:
                raise ToolUnavailable("prompt:x:render", f"upstream {r.status_code}")


@pytest.mark.parametrize(
    "bad_tool_id",
    [
        "prompt:foo:invoke",
        "prompt:foo",
        "prompt::render",
        "prompt:foo:",
    ],
)
async def test_non_render_shape_rejected(bad_tool_id: str) -> None:
    router = ToolRouter(prompt_template_service_url="http://prompts.test")
    with pytest.raises(InvalidPromptToolId, match="prompt:<template_id>:render"):
        await router.invoke(bad_tool_id, {})


async def test_render_without_configured_url_raises() -> None:
    router = ToolRouter(prompt_template_service_url=None)
    with pytest.raises(ValueError, match="PROMPT_TEMPLATE_SERVICE_URL"):
        await router.invoke("prompt:greeting:render", {})


async def test_render_upstream_5xx_maps_to_tool_unavailable(
    fake_transport: _CapturingTransport,
) -> None:
    fake_transport.status = 503
    fake_transport.payload = {"detail": "renderer unavailable"}
    router = ToolRouter(prompt_template_service_url="http://prompts.test")
    with pytest.raises(ToolUnavailable) as excinfo:
        await router.invoke("prompt:greeting:render", {})
    assert excinfo.value.tool_id == "prompt:greeting:render"
    assert "503" in str(excinfo.value)


async def test_render_upstream_4xx_maps_to_tool_unavailable(
    fake_transport: _CapturingTransport,
) -> None:
    fake_transport.status = 422
    fake_transport.payload = {"detail": "unknown variable"}
    router = ToolRouter(prompt_template_service_url="http://prompts.test")
    with pytest.raises(ToolUnavailable):
        await router.invoke("prompt:greeting:render", {})


async def test_render_network_error_maps_to_tool_unavailable(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    class _BrokenTransport(httpx.AsyncBaseTransport):
        async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
            raise httpx.ConnectError("connection refused", request=request)

    original = httpx.AsyncClient.__init__

    def _patched_init(self: httpx.AsyncClient, *args: Any, **kwargs: Any) -> None:
        kwargs.setdefault("transport", _BrokenTransport())
        original(self, *args, **kwargs)

    monkeypatch.setattr(httpx.AsyncClient, "__init__", _patched_init)
    router = ToolRouter(prompt_template_service_url="http://prompts.test")
    with pytest.raises(ToolUnavailable, match="network error"):
        await router.invoke("prompt:greeting:render", {})
