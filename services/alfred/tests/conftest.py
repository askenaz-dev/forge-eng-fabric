"""Shared test fixtures and env defaults.

alfred-litellm-header-injection sets `PROMPT_TEMPLATE_SERVICE_URL` so
`load_settings()` does not raise during test bootstrap. Production refuses
to start without it; tests use a fake value because `LiteLLMClient` and
`ToolRouter` are dependency-injected via `loop_deps`/fixtures.
"""

from __future__ import annotations

import os

os.environ.setdefault("PROMPT_TEMPLATE_SERVICE_URL", "http://localhost:8099")
