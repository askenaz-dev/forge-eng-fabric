"""In-memory store for eval datasets and runs."""

from __future__ import annotations

from threading import RLock

from .models import ABRun, EvalDataset, EvalRun


class Store:
    def __init__(self) -> None:
        self._lock = RLock()
        self._datasets: dict[tuple[str, str], EvalDataset] = {}
        self._runs: dict[str, EvalRun] = {}
        self._ab_runs: dict[str, ABRun] = {}

    # Datasets ---------------------------------------------------------

    def upsert_dataset(self, dataset: EvalDataset) -> EvalDataset:
        with self._lock:
            key = (dataset.asset_id, dataset.version)
            if key in self._datasets:
                # Datasets are immutable per version.
                raise ValueError("dataset_version_already_exists")
            self._datasets[key] = dataset
            return dataset

    def get_dataset(self, asset_id: str, version: str) -> EvalDataset | None:
        with self._lock:
            return self._datasets.get((asset_id, version))

    def list_dataset_versions(self, asset_id: str) -> list[EvalDataset]:
        with self._lock:
            return [d for (a, _), d in self._datasets.items() if a == asset_id]

    def list_datasets(self, tenant_id: str | None = None) -> list[EvalDataset]:
        with self._lock:
            out = list(self._datasets.values())
            if tenant_id:
                out = [d for d in out if d.tenant_id == tenant_id]
            return out

    # Runs -------------------------------------------------------------

    def insert_run(self, run: EvalRun) -> EvalRun:
        with self._lock:
            self._runs[run.id] = run
            return run

    def update_run(self, run: EvalRun) -> EvalRun:
        with self._lock:
            self._runs[run.id] = run
            return run

    def get_run(self, run_id: str) -> EvalRun | None:
        with self._lock:
            return self._runs.get(run_id)

    def list_runs(self, workflow_id: str | None = None) -> list[EvalRun]:
        with self._lock:
            out = list(self._runs.values())
            if workflow_id:
                out = [r for r in out if r.workflow_id == workflow_id]
            out.sort(key=lambda r: r.started_at, reverse=True)
            return out

    def latest_passing_run(self, workflow_id: str, dataset_id: str) -> EvalRun | None:
        with self._lock:
            best = [r for r in self._runs.values()
                    if r.workflow_id == workflow_id and r.dataset_id == dataset_id and r.outcome.value == "passed"]
            if not best:
                return None
            best.sort(key=lambda r: r.completed_at or r.started_at, reverse=True)
            return best[0]

    # A/B --------------------------------------------------------------

    def insert_ab(self, run: ABRun) -> ABRun:
        with self._lock:
            self._ab_runs[run.id] = run
            return run

    def update_ab(self, run: ABRun) -> ABRun:
        with self._lock:
            self._ab_runs[run.id] = run
            return run

    def get_ab(self, run_id: str) -> ABRun | None:
        with self._lock:
            return self._ab_runs.get(run_id)

    def list_ab(self, workflow_id: str | None = None) -> list[ABRun]:
        with self._lock:
            out = list(self._ab_runs.values())
            if workflow_id:
                out = [r for r in out if r.workflow_id == workflow_id]
            out.sort(key=lambda r: r.started_at, reverse=True)
            return out
