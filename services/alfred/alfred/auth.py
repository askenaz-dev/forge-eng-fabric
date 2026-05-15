"""JWT verification (Keycloak) and OpenFGA check helpers.

Lightweight implementation — keys cached per-process. For dev we accept any
RS256 token whose issuer/audience match. Hard failures bubble up as 401.
"""

from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Any

import httpx
import jwt
from jwt import PyJWKClient

_jwks_clients: dict[str, PyJWKClient] = {}


def _client(issuer: str) -> PyJWKClient:
    url = issuer.rstrip("/") + "/protocol/openid-connect/certs"
    if url not in _jwks_clients:
        _jwks_clients[url] = PyJWKClient(url, cache_keys=True, lifespan=3600)
    return _jwks_clients[url]


@dataclass(frozen=True)
class Principal:
    sub: str
    email: str | None
    name: str | None
    roles: tuple[str, ...]
    raw: dict[str, Any]


def verify_jwt(token: str, issuer: str, audience: str) -> Principal:
    if not token:
        raise PermissionError("missing token")
    client = _client(issuer)
    signing_key = client.get_signing_key_from_jwt(token).key
    claims = jwt.decode(
        token,
        signing_key,
        algorithms=["RS256"],
        audience=audience,
        issuer=issuer,
        options={"require": ["exp", "iat"]},
    )
    if claims.get("exp", 0) < time.time():
        raise PermissionError("token expired")
    sub = str(claims.get("sub") or "")
    if not sub:
        raise PermissionError("token has no sub")
    realm_roles = tuple((claims.get("realm_access") or {}).get("roles") or [])
    return Principal(
        sub=sub,
        email=claims.get("email"),
        name=claims.get("preferred_username") or claims.get("name"),
        roles=realm_roles,
        raw=claims,
    )


async def fga_check(
    *,
    base_url: str,
    store_id: str,
    model_id: str,
    user: str,
    relation: str,
    object_: str,
) -> bool:
    """Returns True if the OpenFGA tuple `(user, relation, object)` resolves to allowed."""

    if not store_id:
        return True  # dev fallback when FGA not provisioned
    payload: dict[str, Any] = {
        "tuple_key": {"user": user, "relation": relation, "object": object_}
    }
    if model_id:
        payload["authorization_model_id"] = model_id
    async with httpx.AsyncClient(timeout=5.0) as client:
        r = await client.post(f"{base_url.rstrip('/')}/stores/{store_id}/check", json=payload)
        if r.status_code != 200:
            return False
        return bool(r.json().get("allowed"))


async def _fga_write(
    *,
    base_url: str,
    store_id: str,
    model_id: str,
    writes: list[dict[str, Any]] | None = None,
    deletes: list[dict[str, Any]] | None = None,
) -> None:
    if not store_id:
        return
    payload: dict[str, Any] = {}
    if model_id:
        payload["authorization_model_id"] = model_id
    if writes:
        payload["writes"] = {"tuple_keys": writes}
    if deletes:
        payload["deletes"] = {"tuple_keys": deletes}
    async with httpx.AsyncClient(timeout=5.0) as client:
        await client.post(f"{base_url.rstrip('/')}/stores/{store_id}/write", json=payload)


async def mint_sub_principal(
    *,
    base_url: str,
    store_id: str,
    model_id: str,
    session_id: str,
    workspace_id: str,
) -> str:
    """Mint a sub-principal `system:alfred:session:<uuid>` scoped to platform-readonly.

    Returns the sub-principal name. No-ops when FGA is not provisioned (empty store_id).
    The intersection with system:alfred's capabilities is enforced by the FGA model:
    a session sub-principal can only exercise relations its parent already holds.
    """
    sub = f"system:alfred:session:{session_id}"
    await _fga_write(
        base_url=base_url,
        store_id=store_id,
        model_id=model_id,
        writes=[
            {
                "user": f"user:{sub}",
                "relation": "platform-readonly",
                "object": f"workspace:{workspace_id}",
            }
        ],
    )
    return sub


async def revoke_sub_principal(
    *,
    base_url: str,
    store_id: str,
    model_id: str,
    session_id: str,
    workspace_id: str,
) -> None:
    """Revoke all FGA tuples for a session sub-principal."""
    sub = f"system:alfred:session:{session_id}"
    await _fga_write(
        base_url=base_url,
        store_id=store_id,
        model_id=model_id,
        deletes=[
            {
                "user": f"user:{sub}",
                "relation": "platform-readonly",
                "object": f"workspace:{workspace_id}",
            }
        ],
    )
