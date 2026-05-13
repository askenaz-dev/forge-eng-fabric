"use client";

import { useEffect, useState } from "react";
import { Sheet, Badge } from "@/components/primitives";
import { Check, Clock, Play, X } from "@/components/icons";
import { useLang } from "@/components/providers/LangProvider";

type Step = { ic: string; tone: "ok" | "err" | "em" | "warn"; label: string; ms: string };

type RunDetail = {
  id: string;
  agent: string;
  purpose: string;
  repo: string;
  duration: string;
  policy: string;
  status: string;
  triggered_by: string;
  steps: Step[];
  diff?: { before: string[]; after: string[] };
};

const STEP_ICON = { check: Check, x: X, play: Play, clock: Clock } as const;

export function RunSheet({ id, onClose }: { id: string | null; onClose: () => void }) {
  const { t } = useLang();
  const [detail, setDetail] = useState<RunDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) {
      setDetail(null);
      setError(null);
      return;
    }
    setDetail(null);
    fetch(`/api/sdlc/runs/${encodeURIComponent(id)}`, { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`status ${r.status}`))))
      .then((d: RunDetail) => setDetail(d))
      .catch((err) => setError((err as Error).message));
  }, [id]);

  return (
    <Sheet
      open={id != null}
      onOpenChange={(o) => (o ? null : onClose())}
      title={
        <>
          <em>{t("sheet_run")}</em> · {detail?.purpose ?? id ?? ""}
        </>
      }
      subtitle={detail && `${detail.id} · ${detail.agent} · ${detail.repo}`}
    >
      {error && <div className="note" style={{ textAlign: "center" }}>{error}</div>}
      {detail && (
        <>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 18, marginBottom: 18 }}>
            <div className="field" style={{ borderBottom: 0, padding: 0 }}>
              <span className="k">{t("triggered")}</span>
              <span className="v">{detail.triggered_by}</span>
            </div>
            <div className="field" style={{ borderBottom: 0, padding: 0 }}>
              <span className="k">{t("duration")}</span>
              <span className="v">{detail.duration}</span>
            </div>
            <div className="field" style={{ borderBottom: 0, padding: 0 }}>
              <span className="k">{t("policy_short")}</span>
              <span className="v">{detail.policy}</span>
            </div>
            <div className="field" style={{ borderBottom: 0, padding: 0 }}>
              <span className="k">{t("sheet_policy_pass")}</span>
              <span className="v tag">
                <Badge tone="info" dot>
                  {t("sheet_policy_eval")}
                </Badge>
              </span>
            </div>
          </div>

          <div style={{ marginTop: 6, marginBottom: 8, display: "flex", alignItems: "center", gap: 10 }}>
            <h4 style={{ margin: 0, fontFamily: "var(--f-display)", fontStyle: "italic", fontSize: 18, fontWeight: 400, lineHeight: 1.2 }}>
              {t("sheet_steps")}
            </h4>
            <Badge>
              <Clock />
              <span>{detail.duration}</span>
            </Badge>
          </div>
          <div
            style={{
              background: "var(--bg-sunk)",
              borderRadius: "var(--r-3)",
              padding: "10px 14px",
              fontFamily: "var(--f-mono)",
              fontSize: 12,
            }}
          >
            {detail.steps.map((s, i) => {
              const Icon = STEP_ICON[(s.ic as keyof typeof STEP_ICON) ?? "check"] ?? Check;
              const c = s.tone === "ok" ? "var(--thread)" : s.tone === "err" ? "var(--rust)" : s.tone === "em" ? "var(--primary)" : "var(--spark)";
              return (
                <div
                  key={i}
                  style={{ display: "grid", gridTemplateColumns: "18px 1fr auto", gap: 10, alignItems: "center", padding: "6px 0" }}
                >
                  <span style={{ color: c, display: "inline-flex" }}>
                    <Icon />
                  </span>
                  <span style={{ color: "var(--fg)" }}>{s.label}</span>
                  <span style={{ color: "var(--fg-3)" }}>{s.ms}</span>
                </div>
              );
            })}
          </div>

          {detail.diff && (
            <div style={{ marginTop: 18 }}>
              <h4 style={{ margin: "0 0 8px", fontFamily: "var(--f-display)", fontStyle: "italic", fontSize: 18, fontWeight: 400, lineHeight: 1.2 }}>
                {t("sheet_diff_title")}
              </h4>
              <div className="diff">
                <div className="col">
                  <h5>{t("sheet_diff_before")}</h5>
                  {detail.diff.before.map((l, i) => (
                    <div key={i} className="line">
                      {l}
                    </div>
                  ))}
                </div>
                <div className="col">
                  <h5>{t("sheet_diff_after")}</h5>
                  {detail.diff.after.map((l, i) => (
                    <div key={i} className="line add">
                      {l}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}
        </>
      )}
    </Sheet>
  );
}
