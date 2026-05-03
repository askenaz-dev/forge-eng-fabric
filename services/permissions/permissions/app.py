from __future__ import annotations

from pathlib import Path

from fastapi import FastAPI, HTTPException, Query
from pydantic_settings import BaseSettings, SettingsConfigDict

from permissions.models import CheckRequest, CheckResponse, DelegatedPermission, GrantCreate, RevokeRequest
from permissions.store import FilePermissionStore, InMemoryPermissionStore


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="", extra="ignore")

    permissions_store_path: str = "data/permissions.json"


def create_app(store: InMemoryPermissionStore | None = None, settings: Settings | None = None) -> FastAPI:
    settings = settings or Settings()
    store = store or FilePermissionStore(Path(settings.permissions_store_path))
    app = FastAPI(title="Delegated Permissions", version="0.1.0")
    app.state.store = store

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/v1/permissions/grants", response_model=DelegatedPermission, status_code=201)
    async def create_grant(request: GrantCreate) -> DelegatedPermission:
        return store.create(request)

    @app.get("/v1/permissions/grants")
    async def list_grants(
        subject: str | None = Query(default=None),
        scope_id: str | None = Query(default=None),
        status: str | None = Query(default=None),
    ) -> dict[str, list[DelegatedPermission]]:
        return {"grants": store.list(subject=subject, scope_id=scope_id, status=status)}

    @app.post("/v1/permissions/grants/{grant_id}/revoke", response_model=DelegatedPermission)
    async def revoke_grant(grant_id: str, request: RevokeRequest) -> DelegatedPermission:
        grant = store.revoke(grant_id, request)
        if not grant:
            raise HTTPException(status_code=404, detail="grant not found")
        return grant

    @app.post("/v1/permissions/check", response_model=CheckResponse)
    async def check_permission(request: CheckRequest) -> CheckResponse:
        return store.check(request)

    @app.post("/v1/permissions/expire")
    async def expire_grants() -> dict[str, int]:
        return {"expired": len(store.expire_due())}

    return app


app = create_app()
