"use client";

import { Badge } from "../primitives/Badge";
import { PulseDot, PulseTone } from "../primitives/PulseDot";
import { Chev } from "../icons";
import { useLang } from "../providers/LangProvider";
import { DictKey } from "@/i18n/dictionary";

export type RunStatus = "running" | "success" | "failed" | "pending" | "queued";

export type Run = {
  id: string;
  status: RunStatus;
  agent: string;
  agent_tag: string;
  purpose: string;
  repo: string;
  duration: string;
  policy: string;
};

const STATUS_TO_TONE: Record<RunStatus, PulseTone> = {
  running: "ok",
  success: "ok",
  failed: "err",
  pending: "pending",
  queued: "queued",
};

const STATUS_TO_KEY: Record<RunStatus, DictKey> = {
  running: "st_running",
  success: "st_success",
  failed: "st_failed",
  pending: "st_pending",
  queued: "st_queued",
};

const STATUS_TO_BADGE_TONE = {
  running: "ok",
  success: "ok",
  failed: "err",
  pending: "warn",
  queued: "default",
} as const;

export function RunRow({ run, onOpen }: { run: Run; onOpen?: (run: Run) => void }) {
  const { t } = useLang();
  return (
    <div
      className="run"
      role="button"
      tabIndex={0}
      onClick={() => onOpen?.(run)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpen?.(run);
        }
      }}
    >
      <PulseDot tone={STATUS_TO_TONE[run.status]} />
      <div className="agent">
        <div className="ai">{run.agent_tag}</div>
        <div className="nm">
          {run.purpose}
          <small>
            {run.agent} · {run.id}
          </small>
        </div>
      </div>
      <div className="repo">{run.repo}</div>
      <div className="dur">{run.duration}</div>
      <div>
        <Badge tone={STATUS_TO_BADGE_TONE[run.status]} dot>
          {t(STATUS_TO_KEY[run.status])}
        </Badge>
      </div>
      <div className="pol">{run.policy}</div>
      <Chev className="chev" style={{ width: 14, height: 14 }} />
    </div>
  );
}
