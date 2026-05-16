"use client";

import { useEffect, useState } from "react";
import { Card, CardHeader, Button } from "@/components/primitives";
import { Arrow, Bolt, Check, Play, Shield, Specs } from "@/components/icons";
import { useLang } from "@/components/providers/LangProvider";
import { formatRelativeTime } from "@/i18n/format";
import type { AuditEvent } from "@/app/api/audit/events/route";
import type { DictKey } from "@/i18n/dictionary";
import { useSSE } from "./useSSE";

type EventTone = "ok" | "em" | "warn" | "err";
type Mapping = { key: DictKey; icon: typeof Play; tone: EventTone };

const EVENT_MAP: Record<string, Mapping> = {
  "agent.run.started.v1":         { key: "ev_run_started",   icon: Play,  tone: "em" },
  "approvals.granted.v1":         { key: "ev_apr_granted",   icon: Check, tone: "ok" },
  "policy.denied.v1":             { key: "ev_policy_denied", icon: Shield, tone: "err" },
  "assets.skill.published.v1":    { key: "ev_skill_pub",     icon: Specs,  tone: "em" },
  "self_healing.action.taken.v1": { key: "ev_self_heal",     icon: Bolt,   tone: "em" },
  "openspec.merged.v1":           { key: "ev_spec_merged",   icon: Specs,  tone: "ok" },
};

function renderTemplate(template: string, vars: Record<string, unknown>) {
  const parts = template.split(/(\{\w+\})/g);
  return parts.map((seg, i) => {
    const m = seg.match(/^\{(\w+)\}$/);
    if (m && vars[m[1]] != null) return <code key={i}>{String(vars[m[1]])}</code>;
    return <span key={i}>{seg}</span>;
  });
}

export function ActivityPanel() {
  const { t, lang } = useLang();
  const [events, setEvents] = useState<AuditEvent[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    fetch("/api/audit/events?limit=20&scope=workspace", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`status ${r.status}`))))
      .then((data: { events: AuditEvent[] }) => setEvents(data.events ?? []))
      .catch((err) => setError((err as Error).message));
  }, []);

  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 60_000);
    return () => clearInterval(id);
  }, []);

  // Any audit event prepends to the timeline immediately.
  useSSE(["audit.event.*"], (ev) => {
    const payload = ev.payload as AuditEvent | undefined;
    if (!payload?.id) return;
    setEvents((cur) => [payload, ...(cur ?? [])].slice(0, 20));
  });

  return (
    <Card>
      <CardHeader
        title={t("act_title")}
        sub={t("act_sub")}
        right={
          <Button variant="ghost" size="xs" trailing={<Arrow />}>
            {t("act_view")}
          </Button>
        }
      />
      <div className="timeline">
        {events == null && !error && (
          <>
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="tl-row">
                <div className="skeleton" style={{ width: 22, height: 22, borderRadius: "50%" }} />
                <div className="skeleton" style={{ height: 14, width: "70%" }} />
              </div>
            ))}
          </>
        )}
        {error && (
          <div className="note" style={{ textAlign: "center" }}>
            {error}
          </div>
        )}
        {events !== null && !error && events.length === 0 && (
          <div className="note" style={{ textAlign: "center", color: "var(--fg-3)" }}>
            {t("act_empty")}
          </div>
        )}
        {events?.map((ev) => {
          const mapping = EVENT_MAP[ev.type];
          const tone: EventTone = mapping?.tone ?? "em";
          const Icon = mapping?.icon ?? Play;
          const template = mapping ? t(mapping.key) : ev.type;
          const when = formatRelativeTime(lang, ev.timestamp, now);
          return (
            <div key={ev.id} className="tl-row">
              <div className={`ic ic--${tone}`}>
                <Icon />
              </div>
              <div className="body">
                <div className="txt">{renderTemplate(template, ev.data)}</div>
                <div className="meta">{when}</div>
              </div>
            </div>
          );
        })}
      </div>
    </Card>
  );
}
