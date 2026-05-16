"use client";

import * as Dialog from "@radix-ui/react-dialog";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlfredSessionStatus,
  AlfredStepEvent,
  recordArtifactNavigation,
  useAlfredSession,
} from "./AlfredSessionProvider";
import { useCommandPalette } from "../providers/CommandPaletteProvider";
import { useLang } from "../providers/LangProvider";
import type { DictKey } from "@/i18n/dictionary";
import { cx } from "../primitives/cx";

const MARK_URL = "/alfred-avatar.png";
const MARK_WORKING_URL = "/alfred-avatar.png";

type AlfredPermissions = {
  alfred_invoke: boolean;
  alfred_agent_mode_run: boolean;
};

function isTypingInElement(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false;
  const tag = el.tagName.toLowerCase();
  if (tag === "input" || tag === "textarea" || tag === "select") return true;
  if (el.isContentEditable) return true;
  return false;
}

export function AlfredDock() {
  const { t } = useLang();
  const palette = useCommandPalette();
  const session = useAlfredSession();
  const launcherRef = useRef<HTMLButtonElement | null>(null);
  const [perms, setPerms] = useState<AlfredPermissions>({
    alfred_invoke: false,
    alfred_agent_mode_run: false,
  });

  // Pull permissions for the active workspace.
  useEffect(() => {
    let cancelled = false;
    fetch("/api/permissions/me", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: { alfred_invoke?: boolean; alfred_agent_mode_run?: boolean } | null) => {
        if (!cancelled && data)
          setPerms({
            alfred_invoke: Boolean(data.alfred_invoke),
            alfred_agent_mode_run: Boolean(data.alfred_agent_mode_run),
          });
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  // Hotkey: Alt+A summons the dock; coexists with the palette by closing it
  // when summoned and refusing to fire while typing into a non-dock input.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const isAlfredShortcut = (e.key === "a" || e.key === "A") && e.altKey;
      if (!isAlfredShortcut) return;
      if (isTypingInElement(e.target)) {
        const target = e.target as HTMLElement;
        if (!target.closest(".alfred-dock")) return;
      }
      e.preventDefault();
      if (palette.open) palette.hide();
      session.toggle();
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [palette, session]);

  // Mutual exclusion with palette: opening the palette closes the dock.
  useEffect(() => {
    if (palette.open && session.open) session.hide();
  }, [palette.open, session]);

  const isWorking =
    session.status === "running" || session.status === "planning";
  const isPaused =
    session.status === "paused_for_approval" || session.status === "paused_for_budget";

  if (!perms.alfred_invoke) return null;

  return (
    <>
      <button
        ref={launcherRef}
        type="button"
        className={cx("alfred-launcher", isWorking && "alfred-launcher--working")}
        onClick={session.toggle}
        aria-label={t("alfred_dock_launcher_aria")}
        aria-haspopup="dialog"
        aria-expanded={session.open}
      >
        <AlfredMark working={isWorking} />
        <span className="alfred-launcher__label">{statusLabel(session.status, t)}</span>
      </button>

      <Dialog.Root open={session.open} onOpenChange={(o) => (o ? session.show() : session.hide())}>
        <Dialog.Portal>
          <Dialog.Content
            className="alfred-dock"
            aria-label={t("alfred_dock_aria")}
            aria-describedby={undefined}
            onOpenAutoFocus={(e) => {
              // Focus trap manages first focus; let Radix attempt it then we
              // restore to the launcher on close.
              e.preventDefault();
              const first = document.querySelector<HTMLElement>(".alfred-dock [data-alfred-first]");
              first?.focus();
            }}
            onCloseAutoFocus={(e) => {
              e.preventDefault();
              launcherRef.current?.focus();
            }}
          >
            <Dialog.Title className="sr-only">{t("alfred_dock_aria")}</Dialog.Title>
            <DockHeader paused={isPaused} />
            <DockBody perms={perms} />
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </>
  );
}

function DockHeader({ paused }: { paused: boolean }) {
  const { t } = useLang();
  const session = useAlfredSession();
  return (
    <header className="alfred-dock__header">
      <div className="alfred-dock__title">
        <AlfredMark working={!paused && session.status === "running"} />
        <div>
          <strong>{t("alfred_dock_title")}</strong>
          <small>{statusLabel(session.status, t)}</small>
        </div>
      </div>
      <div className="alfred-dock__actions">
        {session.activeSessionId && session.status && (
          <button
            type="button"
            className="alfred-dock__btn"
            onClick={() => void session.cancel()}
          >
            {t("alfred_dock_cancel")}
          </button>
        )}
        <Dialog.Close asChild>
          <button type="button" className="alfred-dock__btn" aria-label={t("alfred_dock_close")}>
            ×
          </button>
        </Dialog.Close>
      </div>
    </header>
  );
}

function DockBody({ perms }: { perms: AlfredPermissions }) {
  const { t } = useLang();
  const session = useAlfredSession();
  const [intent, setIntent] = useState("");
  const [followUp, setFollowUp] = useState("");
  const [startError, setStartError] = useState<string | null>(null);

  const canStart = perms.alfred_agent_mode_run && intent.trim().length > 0;

  const onStart = useCallback(async () => {
    if (!intent.trim()) return;
    setStartError(null);
    try {
      // workspace_id resolved server-side from the next-auth JWT session.
      await session.start({ workspaceId: "", intent });
      setIntent("");
    } catch (err) {
      setStartError(err instanceof Error ? err.message : t("alfred_err_generic"));
    }
  }, [session, intent, t]);

  const onFollowUp = useCallback(async () => {
    if (!followUp.trim()) return;
    await session.sendFollowUp(followUp);
    setFollowUp("");
  }, [session, followUp]);

  return (
    <div className="alfred-dock__body">
      {!session.activeSessionId && (
        <section className="alfred-dock__section">
          <h3>{t("alfred_dock_compose_title")}</h3>
          <label className="alfred-dock__label">
            <span>{t("alfred_dock_intent_label")}</span>
            <textarea
              data-alfred-first
              className="alfred-dock__input"
              value={intent}
              onChange={(e) => setIntent(e.target.value)}
              rows={3}
              placeholder={t("alfred_dock_intent_placeholder")}
            />
          </label>
          {startError && (
            <p role="alert" className="alfred-dock__error">{startError}</p>
          )}
          <button
            type="button"
            className="alfred-dock__btn alfred-dock__btn--primary"
            disabled={!canStart}
            onClick={() => void onStart()}
            title={
              perms.alfred_agent_mode_run
                ? undefined
                : t("alfred_dock_disabled_no_permission")
            }
          >
            {t("alfred_dock_start")}
          </button>
        </section>
      )}

      {session.activeSessionId && (
        <>
          <PlanChecklist steps={session.steps} />
          <Transcript steps={session.steps} />
          <section className="alfred-dock__section">
            <label className="alfred-dock__label">
              <span>{t("alfred_dock_followup_label")}</span>
              <textarea
                data-alfred-first
                className="alfred-dock__input"
                value={followUp}
                onChange={(e) => setFollowUp(e.target.value)}
                rows={2}
              />
            </label>
            <button
              type="button"
              className="alfred-dock__btn alfred-dock__btn--primary"
              disabled={!followUp.trim()}
              onClick={() => void onFollowUp()}
            >
              {t("alfred_dock_send")}
            </button>
          </section>
          <ArtifactLinks steps={session.steps} />
        </>
      )}
    </div>
  );
}

function PlanChecklist({ steps }: { steps: AlfredStepEvent[] }) {
  const { t } = useLang();
  if (steps.length === 0) {
    return (
      <section className="alfred-dock__section">
        <h3>{t("alfred_dock_plan_title")}</h3>
        <p className="alfred-dock__empty">{t("alfred_dock_plan_empty")}</p>
      </section>
    );
  }
  return (
    <section className="alfred-dock__section">
      <h3>{t("alfred_dock_plan_title")}</h3>
      <ol className="alfred-dock__plan">
        {steps.map((s) => (
          <li key={s.idx} className={cx("alfred-dock__step", `alfred-dock__step--${s.status}`)}>
            <span aria-hidden>{stepGlyph(s.status)}</span>
            <span>{s.summary || `${s.kind} ${s.idx}`}</span>
          </li>
        ))}
      </ol>
    </section>
  );
}

function Transcript({ steps }: { steps: AlfredStepEvent[] }) {
  const { t } = useLang();
  return (
    <section className="alfred-dock__section">
      <h3>{t("alfred_dock_transcript_title")}</h3>
      <ul className="alfred-dock__transcript">
        {steps.map((s) => (
          <li key={`tr-${s.idx}`}>
            <code>step {s.idx}</code> · {s.kind} · {s.status}
          </li>
        ))}
        {steps.length === 0 && <li className="alfred-dock__empty">{t("alfred_dock_transcript_empty")}</li>}
      </ul>
    </section>
  );
}

function ArtifactLinks({ steps }: { steps: AlfredStepEvent[] }) {
  const { t } = useLang();
  const last = steps[steps.length - 1];
  if (!last) return null;
  const summary = last.summary ?? "";
  const links = extractLinks(summary);
  if (links.length === 0) return null;
  return (
    <section className="alfred-dock__section">
      <h3>{t("alfred_dock_artifacts_title")}</h3>
      <ul className="alfred-dock__artifacts">
        {links.map((href) => (
          <li key={href}>
            <a href={href} target="_blank" rel="noreferrer" onClick={() => recordArtifactNavigation(href)}>
              {href}
            </a>
          </li>
        ))}
      </ul>
    </section>
  );
}

function AlfredMark({ working }: { working: boolean }) {
  return (
    <img
      src={working ? MARK_WORKING_URL : MARK_URL}
      alt=""
      aria-hidden
      className={cx("alfred-mark", working && "alfred-mark--working")}
      width={32}
      height={32}
      style={{ borderRadius: "50%", objectFit: "cover" }}
    />
  );
}

function statusLabel(status: AlfredSessionStatus | null, t: (k: DictKey) => string): string {
  switch (status) {
    case "planning":
      return t("alfred_status_planning");
    case "running":
      return t("alfred_status_running");
    case "paused_for_approval":
      return t("alfred_status_paused_approval");
    case "paused_for_budget":
      return t("alfred_status_paused_budget");
    case "completed":
      return t("alfred_status_completed");
    case "aborted":
      return t("alfred_status_aborted");
    case "failed":
      return t("alfred_status_failed");
    default:
      return t("alfred_status_idle");
  }
}

function stepGlyph(status: AlfredStepEvent["status"]): string {
  switch (status) {
    case "succeeded":
      return "◆";
    case "running":
      return "◇";
    case "failed":
      return "◇";
    case "paused_for_approval":
    case "paused_for_budget":
      return "◐";
    case "cancelled":
    case "skipped":
      return "○";
    default:
      return "·";
  }
}

function extractLinks(text: string): string[] {
  const matches = text.match(/https?:\/\/[^\s)]+/g);
  return matches ? Array.from(new Set(matches)) : [];
}
