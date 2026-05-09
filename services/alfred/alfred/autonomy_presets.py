"""Workspace autonomy presets for the Alfred wizard.

Per design D3 of platform-gaps-closure: Workspace admins define a set of named
presets (e.g., `full-autonomy`, `staging-only`, `manual-prod`). Each preset is
an `autonomy_policy` block compatible with `openspec-backbone`. The wizard
surfaces presets as a dropdown with a per-action override panel; overrides are
validated against admin-set ceilings and rejected events are audited.

Storage is a per-Workspace JSON file. Production wiring will move this to
control-plane Postgres without changing the surface (the keys/structure here
are the migration's source-of-truth).
"""

from __future__ import annotations

import json
import threading
import uuid
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

DEFAULT_PRESETS = [
    {
        "name": "full-autonomy",
        "description": "Alfred acts autonomously for all routine actions; deploy:prod still requires approval.",
        "default_mode": "autonomous",
        "approvals_required": ["deploy:prod"],
        "ceilings": {"deploy:prod": "requires_approval", "schema:migrate": "requires_dual_control"},
    },
    {
        "name": "staging-only",
        "description": "Alfred is autonomous in non-prod; everything that touches prod requires approval.",
        "default_mode": "autonomous",
        "approvals_required": ["deploy:prod", "secrets:write", "schema:migrate"],
        "ceilings": {"deploy:prod": "requires_approval", "secrets:write": "requires_dual_control"},
    },
    {
        "name": "manual-prod",
        "description": "Maximum oversight: every prod-relevant action requires explicit approval.",
        "default_mode": "requires_approval",
        "approvals_required": ["deploy:prod", "deploy:staging", "secrets:write", "schema:migrate"],
        "ceilings": {"deploy:prod": "requires_dual_control"},
    },
]


@dataclass
class PresetStore:
    root: Path
    _cache: dict[str, list[dict[str, Any]]] = field(default_factory=dict)
    _lock: threading.RLock = field(default_factory=threading.RLock)

    def __post_init__(self) -> None:
        self.root.mkdir(parents=True, exist_ok=True)

    def _path(self, workspace_id: uuid.UUID) -> Path:
        return self.root / f"{workspace_id}.json"

    def get_or_create(self, workspace_id: uuid.UUID) -> list[dict[str, Any]]:
        key = str(workspace_id)
        with self._lock:
            if key in self._cache:
                return self._cache[key]
            path = self._path(workspace_id)
            if path.exists():
                presets = json.loads(path.read_text(encoding="utf-8"))
            else:
                presets = list(DEFAULT_PRESETS)
                path.write_text(json.dumps(presets, indent=2) + "\n", encoding="utf-8")
            self._cache[key] = presets
            return presets

    def replace(self, workspace_id: uuid.UUID, presets: list[dict[str, Any]]) -> list[dict[str, Any]]:
        key = str(workspace_id)
        with self._lock:
            self._path(workspace_id).write_text(json.dumps(presets, indent=2) + "\n", encoding="utf-8")
            self._cache[key] = presets
        return presets


def validate_override(preset: dict[str, Any], action_class: str, requested_mode: str) -> tuple[bool, str | None]:
    """Validate a per-action override against the preset's ceiling.

    Returns (ok, reason). Modes are ordered (most-permissive → most-restrictive):
      autonomous < requires_approval < requires_dual_control < restricted
    A user MAY make a routine more restrictive than the ceiling but MAY NOT
    make it more permissive.
    """
    order = ["autonomous", "requires_approval", "requires_dual_control", "restricted"]
    ceilings = preset.get("ceilings") or {}
    ceiling = ceilings.get(action_class)
    if ceiling is None:
        return True, None
    try:
        ceiling_idx = order.index(ceiling)
        requested_idx = order.index(requested_mode)
    except ValueError:
        return False, f"unknown autonomy mode: ceiling={ceiling!r} requested={requested_mode!r}"
    if requested_idx >= ceiling_idx:
        return True, None
    return False, (
        f"override violates ceiling: {action_class} ceiling={ceiling!r} requested={requested_mode!r}"
    )
