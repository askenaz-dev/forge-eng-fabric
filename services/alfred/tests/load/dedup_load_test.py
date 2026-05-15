"""Load test for POST /v1/intent/match (spec deduplication retrieval).

SLO target: p95 < 100ms (alfred-console-redesign §9.3 / task 4.6).

Usage:
  # Against local docker-compose stack
  uv run python services/alfred/tests/load/dedup_load_test.py \\
      --url http://localhost:8090 \\
      --workspace-id <uuid> \\
      --token <bearer>

  # Quick sanity run (30 s, 10 VUs)
  uv run python services/alfred/tests/load/dedup_load_test.py \\
      --duration 30 --vus 10

  # Pilot corpus test (90 s, 50 VUs, threshold enforcement)
  uv run python services/alfred/tests/load/dedup_load_test.py \\
      --duration 90 --vus 50 --fail-on-slo-breach

The script prints a summary table and exits non-zero if --fail-on-slo-breach is
set and the p95 > 100ms SLO is breached.
"""
from __future__ import annotations

import argparse
import asyncio
import statistics
import sys
import time
import uuid
from typing import Any

import httpx

DEFAULT_URL = "http://localhost:8090"
DEFAULT_WORKSPACE = str(uuid.uuid4())
SLO_P95_MS = 100.0

SAMPLE_INTENTS = [
    "Create a new user authentication service with OAuth2",
    "Add a rate-limiting layer to the existing API gateway",
    "Build a real-time notification system using WebSockets",
    "Migrate the monolith to microservices with event sourcing",
    "Set up a CI/CD pipeline with automated security scanning",
    "Add multi-tenant data isolation to the platform",
    "Implement a feature-flag system for progressive rollouts",
    "Create a self-healing observability pipeline",
]


async def single_request(
    client: httpx.AsyncClient,
    url: str,
    workspace_id: str,
    intent: str,
    token: str | None,
) -> dict[str, Any]:
    headers: dict[str, str] = {"content-type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"

    start = time.perf_counter()
    try:
        r = await client.post(
            f"{url}/v1/intent/match",
            json={"workspace_id": workspace_id, "text": intent, "k": 5},
            headers=headers,
            timeout=5.0,
        )
        elapsed_ms = (time.perf_counter() - start) * 1000
        return {
            "elapsed_ms": elapsed_ms,
            "status": r.status_code,
            "error": None if r.status_code < 500 else f"HTTP {r.status_code}",
        }
    except Exception as exc:
        elapsed_ms = (time.perf_counter() - start) * 1000
        return {"elapsed_ms": elapsed_ms, "status": 0, "error": str(exc)}


async def virtual_user(
    client: httpx.AsyncClient,
    url: str,
    workspace_id: str,
    token: str | None,
    duration_s: float,
    results: list[dict[str, Any]],
) -> None:
    deadline = time.monotonic() + duration_s
    idx = 0
    while time.monotonic() < deadline:
        intent = SAMPLE_INTENTS[idx % len(SAMPLE_INTENTS)]
        result = await single_request(client, url, workspace_id, intent, token)
        results.append(result)
        idx += 1
        await asyncio.sleep(0)  # yield to event loop


async def run(
    url: str,
    workspace_id: str,
    token: str | None,
    vus: int,
    duration_s: float,
) -> list[dict[str, Any]]:
    results: list[dict[str, Any]] = []
    async with httpx.AsyncClient(http2=True) as client:
        tasks = [
            asyncio.create_task(
                virtual_user(client, url, workspace_id, token, duration_s, results)
            )
            for _ in range(vus)
        ]
        await asyncio.gather(*tasks)
    return results


def print_report(results: list[dict[str, Any]], slo_p95_ms: float) -> bool:
    """Print a latency report. Returns True if SLO is met."""
    total = len(results)
    errors = [r for r in results if r["error"]]
    ok = [r for r in results if not r["error"]]
    latencies = sorted(r["elapsed_ms"] for r in ok)

    def pct(n: float) -> float:
        if not latencies:
            return 0.0
        idx = int(len(latencies) * n / 100)
        return latencies[min(idx, len(latencies) - 1)]

    p50 = pct(50)
    p90 = pct(90)
    p95 = pct(95)
    p99 = pct(99)
    avg = statistics.mean(latencies) if latencies else 0.0
    rps = total / max(1, sum(r["elapsed_ms"] for r in results) / 1000)

    slo_ok = p95 <= slo_p95_ms

    print("\n=== Dedup Retrieval Load Test Results ===")
    print(f"Total requests : {total}")
    print(f"Successful     : {len(ok)}")
    print(f"Errors         : {len(errors)}")
    print(f"Error rate     : {100 * len(errors) / max(1, total):.1f}%")
    print(f"")
    print(f"Latency (ms):")
    print(f"  avg           {avg:.1f}")
    print(f"  p50           {p50:.1f}")
    print(f"  p90           {p90:.1f}")
    print(f"  p95           {p95:.1f}  {'✓ SLO met' if slo_ok else f'✗ SLO BREACHED (target: {slo_p95_ms}ms)'}")
    print(f"  p99           {p99:.1f}")
    print(f"")
    if errors[:3]:
        print("Sample errors:")
        for e in errors[:3]:
            print(f"  {e['error']}")

    return slo_ok


def main() -> int:
    parser = argparse.ArgumentParser(description="Load test for /v1/intent/match")
    parser.add_argument("--url", default=DEFAULT_URL)
    parser.add_argument("--workspace-id", default=DEFAULT_WORKSPACE)
    parser.add_argument("--token", default=None, help="Bearer token (omit for dev mode)")
    parser.add_argument("--vus", type=int, default=20, help="Virtual users (default 20)")
    parser.add_argument("--duration", type=float, default=60.0, help="Duration in seconds (default 60)")
    parser.add_argument(
        "--fail-on-slo-breach",
        action="store_true",
        help="Exit 1 if p95 > 100ms",
    )
    args = parser.parse_args()

    print(f"Target : {args.url}/v1/intent/match")
    print(f"VUs    : {args.vus}")
    print(f"Run for: {args.duration}s")
    print(f"SLO    : p95 < {SLO_P95_MS}ms")

    results = asyncio.run(
        run(args.url, args.workspace_id, args.token, args.vus, args.duration)
    )
    slo_ok = print_report(results, SLO_P95_MS)

    if args.fail_on_slo_breach and not slo_ok:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
