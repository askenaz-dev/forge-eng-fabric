"""Alfred — Forge Control Plane Agent.

Reasoning/action loop, RAG-backed context retrieval, policy evaluation,
tool execution via MCPs/Skills/Prompt Templates, and decision logging.

LLM access is **only** via LiteLLM — direct provider calls are denied at
network and platform policy level.
"""

from alfred.app import create_app

__all__ = ["create_app"]
