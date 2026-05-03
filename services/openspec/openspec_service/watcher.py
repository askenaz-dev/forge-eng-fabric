from __future__ import annotations

import asyncio
from collections.abc import Awaitable, Callable
from pathlib import Path

from openspec_service.store import FilesystemOpenSpecStore

SyncHook = Callable[[FilesystemOpenSpecStore], Awaitable[None] | None]


async def watch_filesystem(
    store: FilesystemOpenSpecStore,
    *,
    interval_seconds: float = 2.0,
    on_sync: SyncHook | None = None,
) -> None:
    """Poll the filesystem source of truth and refresh the query index on change."""

    last_fingerprint = _fingerprint(store.root)
    while True:
        await asyncio.sleep(interval_seconds)
        fingerprint = _fingerprint(store.root)
        if fingerprint == last_fingerprint:
            continue
        store.sync_from_filesystem()
        if on_sync:
            result = on_sync(store)
            if result is not None:
                await result
        last_fingerprint = fingerprint


def sync_once(store: FilesystemOpenSpecStore) -> int:
    store.sync_from_filesystem()
    return len(store.index.rows)


def _fingerprint(root: Path) -> tuple[tuple[str, int, int], ...]:
    if not root.exists():
        return ()
    rows: list[tuple[str, int, int]] = []
    for path in root.glob("*/*.json"):
        stat = path.stat()
        rows.append((str(path.relative_to(root)), int(stat.st_mtime_ns), stat.st_size))
    return tuple(sorted(rows))
