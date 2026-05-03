from __future__ import annotations

from dataclasses import dataclass, field

from prompt_registry.models import PromoteRequest, PromptTemplate, PromptTemplateCreate

THRESHOLDS = {"T0": 0.0, "T1": 0.8, "T2": 0.85, "T3": 0.9, "T4": 0.95, "T5": 0.98}


@dataclass
class InMemoryPromptStore:
    templates: dict[tuple[str, str], PromptTemplate] = field(default_factory=dict)

    def create(self, request: PromptTemplateCreate) -> PromptTemplate:
        key = (request.id, request.version)
        if key in self.templates:
            raise ValueError("template version already exists")
        template = PromptTemplate(**request.model_dump())
        template.change_history.append({"actor": request.owner_team, "action": "published", "version": request.version})
        self.templates[key] = template
        return template

    def get(self, template_id: str, version: str | None = None) -> PromptTemplate | None:
        if version:
            return self.templates.get((template_id, version))
        versions = [item for (tid, _), item in self.templates.items() if tid == template_id]
        return sorted(versions, key=lambda item: item.version)[-1] if versions else None

    def list(self) -> list[PromptTemplate]:
        return sorted(self.templates.values(), key=lambda item: (item.id, item.version))

    def promote(self, template_id: str, version: str, request: PromoteRequest) -> PromptTemplate | None:
        template = self.get(template_id, version)
        if not template:
            return None
        if request.lifecycle_state == "approved":
            threshold = THRESHOLDS[template.trust_level]
            failing = {k: v for k, v in request.eval_scores.items() if v < threshold}
            if failing:
                raise ValueError(f"eval scores below threshold {threshold}: {failing}")
        updated = template.model_copy(deep=True)
        updated.lifecycle_state = request.lifecycle_state
        updated.eval_scores = request.eval_scores
        updated.change_history.append({"actor": request.actor, "action": f"promoted:{request.lifecycle_state}"})
        self.templates[(template_id, version)] = updated
        return updated
