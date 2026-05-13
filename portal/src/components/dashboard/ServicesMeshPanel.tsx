"use client";

import { useEffect, useState } from "react";
import { Card, CardHeader, Badge } from "@/components/primitives";
import { useLang } from "@/components/providers/LangProvider";
import type { ServiceHealth, ServicesHealthPayload } from "@/app/api/observability/services/health/route";

const POSITIONS: Record<string, { x: number; y: number }> = {
  orchestrator:    { x: 220, y: 110 },
  "policy-svc":    { x:  60, y:  40 },
  openfga:         { x:  60, y: 110 },
  registry:        { x:  60, y: 180 },
  audit:           { x: 220, y: 200 },
  "context-eng":   { x: 380, y: 180 },
  "spec-engine":   { x: 380, y: 110 },
  pgvector:        { x: 380, y:  40 },
};

function color(state: ServiceHealth["state"]) {
  if (state === "down") return "var(--rust)";
  if (state === "degraded") return "var(--spark)";
  return "var(--thread)";
}

export function ServicesMeshPanel() {
  const { t } = useLang();
  const [data, setData] = useState<ServicesHealthPayload | null>(null);
  useEffect(() => {
    fetch("/api/observability/services/health", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`status ${r.status}`))))
      .then((d: ServicesHealthPayload) => setData(d))
      .catch(() => setData({ services: [] }));
  }, []);

  const services = data?.services ?? [];
  const healthyCount = services.filter((s) => s.state === "healthy").length;
  const degradedCount = services.filter((s) => s.state === "degraded").length;
  const downCount = services.filter((s) => s.state === "down").length;

  return (
    <Card className="mesh-card">
      <CardHeader
        title={t("svc_title")}
        sub={t("svc_sub")}
        right={
          <>
            {healthyCount > 0 && (
              <Badge tone="ok" dot>
                {healthyCount} {t("svc_healthy")}
              </Badge>
            )}
            {degradedCount > 0 && (
              <Badge tone="warn" dot>
                {degradedCount} {t("svc_degraded")}
              </Badge>
            )}
            {downCount > 0 && (
              <Badge tone="err" dot>
                {downCount}
              </Badge>
            )}
          </>
        }
      />
      <svg viewBox="0 0 440 220" preserveAspectRatio="xMidYMid meet" aria-hidden="true">
        <defs>
          <radialGradient id="ringGlow" cx="50%" cy="50%" r="50%">
            <stop offset="0%" stopColor="var(--primary)" stopOpacity="0.18" />
            <stop offset="60%" stopColor="var(--primary)" stopOpacity="0" />
          </radialGradient>
        </defs>
        <circle cx={220} cy={110} r="80" fill="url(#ringGlow)" />
        {services
          .filter((s) => s.id !== "orchestrator")
          .map((s) => {
            const p = POSITIONS[s.id];
            if (!p) return null;
            const c = color(s.state);
            return (
              <g key={s.id}>
                <line x1={220} y1={110} x2={p.x} y2={p.y} stroke="var(--border-strong)" strokeWidth="1" strokeDasharray="2 3" />
                <circle cx={p.x} cy={p.y} r="6" fill="var(--bg-card)" stroke={c} strokeWidth="1.5" />
                <circle cx={p.x} cy={p.y} r="2.5" fill={c}>
                  <animate attributeName="opacity" values="0.4;1;0.4" dur="2.4s" repeatCount="indefinite" />
                </circle>
                <text x={p.x} y={p.y - 11} fontFamily="var(--f-mono)" fontSize="9" fill="var(--fg-2)" textAnchor="middle" letterSpacing="0.04em">
                  {s.id}
                </text>
              </g>
            );
          })}
        <circle cx={220} cy={110} r="22" fill="var(--bg-card)" stroke="var(--primary)" strokeWidth="1.5" />
        <circle cx={220} cy={110} r="6" fill="var(--primary)" />
        <text x={220} y={148} fontFamily="var(--f-mono)" fontSize="9.5" fill="var(--fg)" textAnchor="middle" letterSpacing="0.08em">
          ORCHESTRATOR
        </text>
      </svg>
      <div className="svc-grid">
        {services.slice(0, 4).map((s) => (
          <div className="svc-tile" key={s.id}>
            <div className="nm">{s.id}</div>
            <div className="kind">{t(`svck_${kindKey(s.kind)}` as Parameters<typeof t>[0])}</div>
            <div className="met">
              <div className="ms">
                {s.rps ?? "—"}
                <small> rps</small>
              </div>
              <Badge tone={s.state === "healthy" ? "ok" : s.state === "degraded" ? "warn" : "err"} dot>
                {s.p99 ?? "—"}
              </Badge>
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}

function kindKey(kind: string): string {
  switch (kind) {
    case "orchestration": return "orch";
    case "policy":        return "policy";
    case "authz":         return "authz";
    case "registry":      return "registry";
    case "audit":         return "audit";
    case "context":       return "ctx";
    case "spec":          return "spec";
    case "db":
    case "data":          return "db";
    default:              return "orch";
  }
}
