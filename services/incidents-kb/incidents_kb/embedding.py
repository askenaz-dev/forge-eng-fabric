"""Lightweight bag-of-words embedding for tests / synthetic flows.

Production wires this to the same embeddings model used by the platform RAG
ingest service (registered in `services/rag-ingest`). For tests, a deterministic
character-frequency vector is sufficient: it gives reproducible cosine
similarity between strings without external dependencies.
"""

from __future__ import annotations

import math
import re
from collections import Counter

_TOKEN_RE = re.compile(r"[a-z0-9]+")


def tokenize(text: str) -> list[str]:
    return _TOKEN_RE.findall(text.lower())


def embed(text: str, dim: int = 64) -> list[float]:
    """Hash tokens into `dim` buckets and L2-normalise."""
    bucket = [0.0] * dim
    for token in tokenize(text):
        idx = hash(token) % dim
        bucket[idx] += 1.0
    norm = math.sqrt(sum(v * v for v in bucket))
    if norm == 0:
        return bucket
    return [v / norm for v in bucket]


def cosine(a: list[float], b: list[float]) -> float:
    if len(a) != len(b):
        return 0.0
    return sum(x * y for x, y in zip(a, b, strict=False))
