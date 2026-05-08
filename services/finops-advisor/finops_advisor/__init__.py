"""Forge FinOps advisor (Phase 6)."""

from .advisor import FinOpsAdvisor
from .events import LogSink, MemorySink, Sink
from .models import (
    CostRecord,
    Recommendation,
    RecommendationKind,
    RecommendationsRequest,
)
from .patterns import (
    CacheablePromptDetector,
    DailyCostQuery,
    ExpensiveLLMSkillDetector,
    IdleResourceDetector,
    OversizedResourceDetector,
    PatternDetector,
)

__all__ = [
    "CacheablePromptDetector",
    "CostRecord",
    "DailyCostQuery",
    "ExpensiveLLMSkillDetector",
    "FinOpsAdvisor",
    "IdleResourceDetector",
    "LogSink",
    "MemorySink",
    "OversizedResourceDetector",
    "PatternDetector",
    "Recommendation",
    "RecommendationKind",
    "RecommendationsRequest",
    "Sink",
]
