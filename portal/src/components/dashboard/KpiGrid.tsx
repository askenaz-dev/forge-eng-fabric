"use client";

import { useCallback, useEffect, useState } from "react";
import { Kpi, KpiSkeleton } from "@/components/primitives";
import { Bolt, Check, Clock, Workflows as WorkflowsIcon } from "@/components/icons";
import { useLang } from "@/components/providers/LangProvider";
import { formatNumber } from "@/i18n/format";
import type { KpiPayload } from "@/app/api/observability/kpis/route";
import { useSSE } from "./useSSE";

export function KpiGrid() {
  const { t, lang } = useLang();
  const [data, setData] = useState<KpiPayload | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    fetch("/api/observability/kpis?window=24h", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`status ${r.status}`))))
      .then((payload: KpiPayload) => setData(payload))
      .catch((err) => setError((err as Error).message));
  }, []);

  useEffect(() => { load(); }, [load]);

  useSSE(["observability.kpi.updated", "agent.run.started.v1", "agent.run.completed.v1"], () => load());

  if (!data && !error) {
    return (
      <div className="kpi-grid">
        <KpiSkeleton />
        <KpiSkeleton />
        <KpiSkeleton />
        <KpiSkeleton />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="kpi-grid">
        <Kpi label={t("kpi_runs")} icon={WorkflowsIcon} num="—" foot={t("no_data")} />
        <Kpi label={t("kpi_success")} icon={Check} num="—" foot={t("no_data")} />
        <Kpi label={t("kpi_p95")} icon={Clock} num="—" foot={t("no_data")} />
        <Kpi label={t("kpi_savings")} icon={Bolt} num="—" foot={t("no_data")} />
      </div>
    );
  }

  return (
    <div className="kpi-grid">
      <Kpi
        label={t("kpi_runs")}
        icon={WorkflowsIcon}
        num={formatNumber(lang, data.runs_in_flight.value)}
        delta={
          data.runs_in_flight.delta_pct != null
            ? { dir: data.runs_in_flight.delta_pct >= 0 ? "up" : "down", v: `${data.runs_in_flight.delta_pct >= 0 ? "+" : ""}${data.runs_in_flight.delta_pct}%` }
            : undefined
        }
        foot={t("kpi_vs")}
        data={data.runs_in_flight.samples}
        color="var(--primary)"
      />
      <Kpi
        label={t("kpi_success")}
        icon={Check}
        num={formatNumber(lang, data.success_rate_24h.value, { maximumFractionDigits: 1, minimumFractionDigits: 1 })}
        unit="%"
        delta={
          data.success_rate_24h.delta_pts != null
            ? { dir: data.success_rate_24h.delta_pts >= 0 ? "up" : "down", v: `${data.success_rate_24h.delta_pts >= 0 ? "+" : ""}${data.success_rate_24h.delta_pts} pts` }
            : undefined
        }
        foot={t("kpi_vs")}
        data={data.success_rate_24h.samples}
        color="var(--thread)"
      />
      <Kpi
        label={t("kpi_p95")}
        icon={Clock}
        num={data.p95_ms.value == null ? "—" : formatNumber(lang, data.p95_ms.value)}
        unit="ms"
        delta={
          data.p95_ms.delta_ms != null
            ? { dir: data.p95_ms.delta_ms <= 0 ? "up" : "down", v: `${data.p95_ms.delta_ms <= 0 ? "−" : "+"}${Math.abs(data.p95_ms.delta_ms)}ms` }
            : undefined
        }
        foot={t("avg_today")}
        data={data.p95_ms.samples}
        color="var(--info)"
      />
      <Kpi
        label={t("kpi_savings")}
        icon={Bolt}
        num={formatNumber(lang, data.hours_saved.value)}
        unit="h"
        delta={
          data.hours_saved.delta_h != null
            ? { dir: data.hours_saved.delta_h >= 0 ? "up" : "down", v: `${data.hours_saved.delta_h >= 0 ? "+" : ""}${data.hours_saved.delta_h}h` }
            : undefined
        }
        foot={t("kpi_vs")}
        data={data.hours_saved.samples}
        color="var(--copper)"
      />
    </div>
  );
}
