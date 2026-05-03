from __future__ import annotations

import time
from collections.abc import Callable
from typing import Any

import jsonschema

from reference_skills.schemas import OUTPUT_SCHEMAS


def run_eval_suite(skill_id: str, fn: Callable[..., dict[str, Any]], *args, **kwargs) -> dict[str, Any]:
    start = time.perf_counter()
    output = fn(*args, **kwargs)
    latency_ms = (time.perf_counter() - start) * 1000
    jsonschema.validate(output, OUTPUT_SCHEMAS[skill_id])
    return {
        "skill_id": skill_id,
        "passed": True,
        "scores": {
            "schema": 1.0,
            "latency": 1.0 if latency_ms < 250 else 0.5,
            "cost": 1.0,
            "safety": 1.0,
        },
        "latency_ms": latency_ms,
    }
