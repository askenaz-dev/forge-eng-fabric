"use client";

/**
 * Spec-dedup match dialog (alfred-console-redesign §6).
 *
 * Rendered by both Friendly and Advanced views, the wizard, and the dock.
 * Action ordering:
 *   - lifecycle_state in {approved, committed}: Implementar (primary), Extender, Crear nuevo
 *   - otherwise: Extender (primary), Crear nuevo, Ver otros similares
 */

import * as Dialog from "@radix-ui/react-dialog";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { useLang } from "@/components/providers/LangProvider";

const COMMITTED_STATES = new Set(["approved", "committed"]);

export interface SpecMatchCandidate {
  spec_id: string;
  title: string;
  score: number;
  lifecycle_state: string;
  summary: string;
}

export interface SpecMatch {
  candidate: SpecMatchCandidate;
  threshold: number;
}

interface MatchDialogProps {
  match: SpecMatch;
  workspaceId: string;
  view: "friendly" | "advanced";
  onDismiss: () => void;
}

export function MatchDialog({ match, workspaceId, view, onDismiss }: MatchDialogProps) {
  const { t } = useLang();
  const router = useRouter();
  const [loading, setLoading] = useState<string | null>(null);
  const { candidate } = match;
  const isCommitted = COMMITTED_STATES.has(candidate.lifecycle_state);

  async function handleImplementar() {
    setLoading("implement");
    try {
      // 6.3: wire to POST /v1/agent-mode/sessions with start_step=architect.
      const r = await fetch("/api/alfred/sessions", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workspace_id: workspaceId,
          openspec_id: candidate.spec_id,
          intent: `Implement ${candidate.title}`,
          start_step: "architect",
        }),
      });
      if (!r.ok) throw new Error(await r.text());
      const { session_id } = await r.json() as { session_id: string };
      onDismiss();
      // Route to the session detail page.
      router.push(`/alfred/sessions/${session_id}`);
    } catch (err) {
      console.error("implement failed", err);
    } finally {
      setLoading(null);
    }
  }

  async function handleExtender() {
    setLoading("extend");
    try {
      // 6.4: wire to POST /v1/intent/start with resume_spec_id.
      const r = await fetch("/api/alfred/intent/start", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workspace_id: workspaceId,
          resume_spec_id: candidate.spec_id,
          business_intent: `Extend ${candidate.title}`,
          view,
        }),
      });
      if (!r.ok) throw new Error(await r.text());
      onDismiss();
      router.push(`/alfred/wizard?wizard=1&workspace_id=${workspaceId}&resume_spec_id=${candidate.spec_id}`);
    } catch (err) {
      console.error("extend failed", err);
    } finally {
      setLoading(null);
    }
  }

  async function handleCrearNuevo() {
    setLoading("create");
    try {
      // 6.5: wire to POST /v1/intent/start with bypass_match=true.
      const r = await fetch("/api/alfred/intent/start", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workspace_id: workspaceId,
          business_intent: "New intent",
          bypass_match: true,
          view,
        }),
      });
      if (!r.ok) throw new Error(await r.text());
      const body = await r.json();
      onDismiss();
      if (body.draft?.draft_id) {
        router.push(`/alfred/wizard?wizard=1&workspace_id=${workspaceId}&draft_id=${body.draft.draft_id}`);
      } else {
        router.push(`/alfred?view=${view}`);
      }
    } catch (err) {
      console.error("create new failed", err);
    } finally {
      setLoading(null);
    }
  }

  async function handleNotSame() {
    // 6.6: emit alfred.intent.match_dismissed.v1.
    setLoading("dismiss");
    try {
      await fetch("/api/alfred/intent/match-dismissed", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          spec_id: candidate.spec_id,
          score: candidate.score,
          workspace_id: workspaceId,
          view,
        }),
        keepalive: true,
      });
    } catch {
      // best-effort
    } finally {
      setLoading(null);
      onDismiss();
    }
  }

  const scorePercent = Math.round(candidate.score * 100);

  return (
    <Dialog.Root open onOpenChange={(open) => !open && onDismiss()}>
      <Dialog.Portal>
        <Dialog.Overlay className="scrim" />
        <Dialog.Content className="modal" aria-describedby="match-desc">
          <Dialog.Title className="modal-title">{t("alfred_match_title")}</Dialog.Title>
          <p id="match-desc" className="modal-sub">{t("alfred_match_subtitle")}</p>

          <div className="match-candidate">
            <div className="match-candidate-title">
              <em>{candidate.title}</em>
            </div>
            {candidate.summary && (
              <p className="match-candidate-summary text-sm" style={{ color: "var(--fg-2)" }}>
                {candidate.summary}
              </p>
            )}
            <div className="match-candidate-meta text-xs" style={{ color: "var(--fg-3)", fontFamily: "var(--f-mono)" }}>
              <span>{t("alfred_match_score")}: {scorePercent}%</span>
              <span style={{ marginLeft: 12 }}>{t("alfred_match_lifecycle")}: {candidate.lifecycle_state}</span>
            </div>
          </div>

          {/* 6.2: action ordering */}
          <div className="modal-actions">
            {isCommitted ? (
              <>
                <button
                  type="button"
                  className="btn-primary"
                  onClick={handleImplementar}
                  disabled={loading !== null}
                >
                  {loading === "implement" ? "…" : t("alfred_match_implement")}
                </button>
                <button
                  type="button"
                  className="btn-secondary"
                  onClick={handleExtender}
                  disabled={loading !== null}
                >
                  {loading === "extend" ? "…" : t("alfred_match_extend")}
                </button>
                <button
                  type="button"
                  className="btn-ghost"
                  onClick={handleCrearNuevo}
                  disabled={loading !== null}
                >
                  {loading === "create" ? "…" : t("alfred_match_create_new")}
                </button>
              </>
            ) : (
              <>
                <button
                  type="button"
                  className="btn-primary"
                  onClick={handleExtender}
                  disabled={loading !== null}
                >
                  {loading === "extend" ? "…" : t("alfred_match_extend")}
                </button>
                <button
                  type="button"
                  className="btn-secondary"
                  onClick={handleCrearNuevo}
                  disabled={loading !== null}
                >
                  {loading === "create" ? "…" : t("alfred_match_create_new")}
                </button>
                <button
                  type="button"
                  className="btn-ghost text-sm"
                  onClick={() => {/* TODO: expand remaining candidates */}}
                  style={{ color: "var(--fg-3)" }}
                >
                  {t("alfred_match_see_others")}
                </button>
              </>
            )}
          </div>

          <div className="modal-footer">
            <button
              type="button"
              className="btn-ghost text-sm"
              onClick={handleNotSame}
              disabled={loading !== null}
              style={{ color: "var(--fg-3)" }}
            >
              {t("alfred_match_not_same")}
            </button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
