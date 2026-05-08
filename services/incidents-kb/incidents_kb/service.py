"""Incidents KB service.

This module provides:
  - vectorisation of resolved incidents (summary + symptoms + root cause).
  - per-tenant collection storage in memory (Milvus collection `incidents-kb-{tenant}`
    in production).
  - top-K similarity search consumed by the diagnosis pipeline.
  - a clustering job that flags recurrent incidents and emits
    `incident.recurrent.detected.v1`.
"""

from __future__ import annotations

import uuid
from collections import defaultdict

from .embedding import cosine, embed
from .events import Sink, new_event
from .models import IndexRequest, KBEntry, RecurrentCluster, SimilarRequest, SimilarResult


class IncidentsKB:
    def __init__(self, sink: Sink) -> None:
        self.sink = sink
        self._collections: dict[str, list[KBEntry]] = defaultdict(list)
        self._cluster_threshold = 0.85
        self._recurrent_min_occurrences = 3

    def index(self, req: IndexRequest) -> KBEntry:
        text = f"{req.summary}\n\n{req.symptoms}\n\nRoot cause: {req.root_cause}"
        embedding = embed(text)
        entry = KBEntry(
            incident_id=req.incident_id,
            tenant_id=req.tenant_id,
            workspace_id=req.workspace_id,
            service=req.service,
            environment=req.environment,
            summary=req.summary,
            symptoms=req.symptoms,
            root_cause=req.root_cause,
            healing_actions=list(req.healing_actions),
            embedding=embedding,
            synthetic=req.synthetic,
        )
        self._collections[req.tenant_id].append(entry)
        self.sink.emit(
            new_event(
                tenant_id=req.tenant_id,
                workspace_id=req.workspace_id,
                event_type="kb.incident.indexed.v1",
                subject=f"incident/{req.incident_id}",
                data={
                    "incident_id": req.incident_id,
                    "service": req.service,
                    "environment": req.environment,
                    "tenant_id": req.tenant_id,
                    "synthetic": req.synthetic,
                },
            )
        )
        return entry

    def similar(self, req: SimilarRequest) -> list[SimilarResult]:
        embedding = embed(req.query)
        out: list[SimilarResult] = []
        for entry in self._collections.get(req.tenant_id, []):
            if req.service and entry.service != req.service:
                continue
            if req.environment and entry.environment != req.environment:
                continue
            score = cosine(embedding, entry.embedding)
            if score < req.min_score:
                continue
            out.append(
                SimilarResult(
                    incident_id=entry.incident_id,
                    score=score,
                    summary=entry.summary,
                    root_cause=entry.root_cause,
                    healing_actions=list(entry.healing_actions),
                )
            )
        out.sort(key=lambda r: r.score, reverse=True)
        return out[: req.top_k]

    def detect_recurrent(self, tenant_id: str) -> list[RecurrentCluster]:
        """Single-link clustering over the collection.

        For Phase 6 we use a threshold-based agglomeration (cheap, deterministic).
        Production swaps this for Milvus IVF clustering or dedicated clustering
        jobs.
        """
        entries = self._collections.get(tenant_id, [])
        clusters: list[list[KBEntry]] = []
        for entry in entries:
            placed = False
            for cluster in clusters:
                if cosine(entry.embedding, cluster[0].embedding) >= self._cluster_threshold:
                    cluster.append(entry)
                    placed = True
                    break
            if not placed:
                clusters.append([entry])

        recurrent: list[RecurrentCluster] = []
        for cluster in clusters:
            if len(cluster) < self._recurrent_min_occurrences:
                continue
            ids = [c.incident_id for c in cluster]
            cluster_id = "rec-" + str(uuid.uuid4())
            recurrent.append(
                RecurrentCluster(
                    cluster_id=cluster_id,
                    tenant_id=tenant_id,
                    incidents=ids,
                    representative_summary=cluster[0].summary,
                    occurrence_count=len(cluster),
                )
            )
            self.sink.emit(
                new_event(
                    tenant_id=tenant_id,
                    workspace_id=None,
                    event_type="incident.recurrent.detected.v1",
                    subject=f"cluster/{cluster_id}",
                    data={
                        "cluster_id": cluster_id,
                        "tenant_id": tenant_id,
                        "incident_ids": ids,
                        "representative_summary": cluster[0].summary,
                        "occurrence_count": len(cluster),
                    },
                )
            )
        return recurrent
