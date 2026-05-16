"use client";

import { useEffect, useMemo, useState } from "react";
import { Card, CardHeader, Chip, ChipRow, Button } from "@/components/primitives";
import { Filter, Refresh } from "@/components/icons";
import { useLang } from "@/components/providers/LangProvider";
import { RunRow, type Run, type RunStatus } from "@/components/runs/RunRow";
import { useSSE } from "./useSSE";

type FilterId = "all" | RunStatus;

const STATUS_FILTERS: Array<{ id: FilterId; labelKey: Parameters<ReturnType<typeof useLang>["t"]>[0] }> = [
  { id: "all",     labelKey: "runs_filter_all" },
  { id: "running", labelKey: "runs_filter_running" },
  { id: "pending", labelKey: "runs_filter_wait" },
  { id: "success", labelKey: "runs_filter_succ" },
  { id: "failed",  labelKey: "runs_filter_failed" },
];

export function RunsPanel({ onOpen }: { onOpen?: (run: Run) => void }) {
  const { t } = useLang();
  const [runs, setRuns] = useState<Run[] | null>(null);
  const [filter, setFilter] = useState<FilterId>("all");
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  function load(manual = false) {
    if (manual) setRefreshing(true);
    setError(null);
    fetch("/api/sdlc/runs?limit=50&order=desc", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`status ${r.status}`))))
      .then((data: { runs: Run[] }) => setRuns(data.runs ?? []))
      .catch((err) => { setError((err as Error).message); setRuns([]); })
      .finally(() => setRefreshing(false));
  }

  useEffect(load, []);

  // Live updates: prepend new runs, refresh the list on status changes.
  useSSE(["agent.run.started.v1", "agent.run.completed.v1", "agent.run.failed.v1"], (ev) => {
    const payload = ev.payload as { run?: Run; run_id?: string } | undefined;
    if (ev.type === "agent.run.started.v1" && payload?.run) {
      setRuns((cur) => (cur ? [payload.run!, ...cur].slice(0, 50) : cur));
      return;
    }
    // For completed / failed events, re-fetch to pull canonical status.
    load();
  });

  const counts = useMemo(() => {
    const base = { all: 0, running: 0, success: 0, failed: 0, pending: 0, queued: 0 };
    for (const r of runs ?? []) {
      base.all += 1;
      base[r.status] += 1;
    }
    return base;
  }, [runs]);

  const visible = useMemo(() => {
    if (!runs) return [];
    return filter === "all" ? runs : runs.filter((r) => r.status === filter);
  }, [runs, filter]);

  return (
    <Card>
      <CardHeader
        title={t("runs_title")}
        sub={t("runs_sub")}
        right={
          <>
            <span className="live-now">
              <span className="lvd" /> {t("h_live")}
            </span>
            <Button variant="ghost" size="xs" onClick={() => load(true)} aria-label={t("refresh")} disabled={refreshing}>
              {refreshing ? <span className="spinner" /> : <Refresh />}
            </Button>
          </>
        }
      />
      <ChipRow>
        {STATUS_FILTERS.map((f) => (
          <Chip
            key={f.id}
            pressed={filter === f.id}
            onClick={() => setFilter(f.id)}
            count={counts[f.id]}
          >
            {t(f.labelKey)}
          </Chip>
        ))}
        <Button variant="ghost" size="xs" leading={<Filter />} style={{ marginLeft: "auto" }}>
          {t("runs_filters")}
        </Button>
      </ChipRow>
      <div className="runs">
        {runs == null
          ? Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="run">
                <div className="skeleton" style={{ width: 8, height: 8, borderRadius: 999 }} />
                <div className="skeleton" style={{ height: 22, width: "70%" }} />
                <div className="skeleton" style={{ height: 12, width: "60%" }} />
                <div className="skeleton" style={{ height: 12, width: 50 }} />
                <div className="skeleton" style={{ height: 18, width: 70, borderRadius: 999 }} />
                <div className="skeleton" style={{ height: 12, width: 60 }} />
                <div className="skeleton" style={{ height: 12, width: 14 }} />
              </div>
            ))
          : visible.length === 0
            ? (
              <div className="note" style={{ padding: "32px 18px", textAlign: "center" }}>
                {error ?? t("apr_no_items")}
              </div>
            )
            : visible.map((run) => <RunRow key={run.id} run={run} onOpen={onOpen} />)}
      </div>
    </Card>
  );
}
