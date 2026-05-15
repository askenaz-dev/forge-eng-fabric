"use client";

/**
 * Alfred Friendly View (alfred-console-redesign §1).
 *
 * Landing surface with three cards: Nueva App / Mejorar / Operar.
 * Selecting a card opens a scoped conversation panel.
 * Raw IDs are never rendered; all entity references use human labels.
 */

import { useState, useEffect } from "react";
import { useLang } from "@/components/providers/LangProvider";
import { useToast } from "@/components/providers/ToastProvider";
import { AppSwitcher, type AppEntry } from "./AppSwitcher";
import { mapErrorCode } from "./errorMessages";

type CardKind = "new_app" | "improve_app" | "operate_app";

interface Message {
  role: "user" | "assistant";
  content: string;
}

interface FriendlyViewProps {
  apps: AppEntry[];
  activeAppId: string | null;
  workspaceId: string;
  onSwitchView: () => void;
}

interface HealingEvent {
  id: string;
  type: "l1" | "l2";
  app_id: string;
  summary: string;
  severity: string;
  created_at: string;
}

const PROMPT_TEMPLATES: Record<CardKind, string> = {
  new_app: "Quiero crear una nueva App.",
  improve_app: "Quiero mejorar o extender una App existente.",
  operate_app: "Quiero desplegar, supervisar o resolver un problema en una App.",
};

export function FriendlyView({
  apps,
  activeAppId: initialAppId,
  workspaceId,
  onSwitchView,
}: FriendlyViewProps) {
  const { t } = useLang();
  const toast = useToast();
  const [activeCard, setActiveCard] = useState<CardKind | null>(null);
  const [activeAppId, setActiveAppId] = useState<string | null>(initialAppId);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [draftId, setDraftId] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<{ friendly: string; raw: string } | null>(null);
  const [showRaw, setShowRaw] = useState(false);
  // sdlc-end-to-end (10.1): track committed intent data for workflow trigger
  const [committedIntent, setCommittedIntent] = useState<{ openspec_id: string; app_id: string } | null>(null);
  const [workflowStarted, setWorkflowStarted] = useState(false);
  // sdlc-end-to-end (10.2): healing events for Operar card
  const [healingEvents, setHealingEvents] = useState<HealingEvent[]>([]);
  const [healingLoading, setHealingLoading] = useState(false);
  const [healingError, setHealingError] = useState(false);

  const cards: { kind: CardKind; title: string; body: string }[] = [
    { kind: "new_app", title: t("alfred_card_new_app"), body: t("alfred_card_new_app_body") },
    { kind: "improve_app", title: t("alfred_card_improve"), body: t("alfred_card_improve_body") },
    { kind: "operate_app", title: t("alfred_card_operate"), body: t("alfred_card_operate_body") },
  ];

  // Fetch healing events when the Operar card is active.
  useEffect(() => {
    if (activeCard !== "operate_app") return;
    setHealingLoading(true);
    setHealingError(false);
    void fetch(`/api/healing/events?app_id=${activeAppId ?? ""}&limit=10`, { cache: "no-store" })
      .then((r) => r.ok ? r.json() : Promise.reject(r.status))
      .then((data: { events?: HealingEvent[] }) => {
        setHealingEvents(data.events ?? []);
      })
      .catch(() => setHealingError(true))
      .finally(() => setHealingLoading(false));
  }, [activeCard, activeAppId]);

  // Trigger forge.reference.intent-to-infrastructure@1 after intent commit.
  async function launchIntentToInfrastructure(openspecId: string, appId: string) {
    setLoading(true);
    try {
      const r = await fetch("/api/workflow/runs", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workflow_id: "forge.reference.intent-to-infrastructure@1",
          inputs: {
            app_id: appId,
            openspec_id: openspecId,
            correlation_id: crypto.randomUUID(),
            include: ["iac", "observability"],
          },
        }),
      });
      if (!r.ok) {
        const text = await r.text();
        setError({ friendly: t("alfred_new_app_workflow_err"), raw: text });
        return;
      }
      setWorkflowStarted(true);
      setMessages((prev) => [
        ...prev,
        { role: "assistant", content: t("alfred_new_app_workflow_started") },
      ]);
    } catch (err) {
      setError({ friendly: t("alfred_new_app_workflow_err"), raw: String(err) });
    } finally {
      setLoading(false);
    }
  }

  async function startCard(kind: CardKind) {
    setActiveCard(kind);
    setMessages([]);
    setDraftId(null);
    setError(null);
    setCommittedIntent(null);
    setWorkflowStarted(false);
    setLoading(true);

    try {
      const r = await fetch("/api/alfred/intent/start", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          workspace_id: workspaceId,
          app_id: activeAppId,
          business_intent: PROMPT_TEMPLATES[kind],
          view: "friendly",
        }),
      });
      if (!r.ok) {
        const text = await r.text();
        const key = mapErrorCode(`${r.status} ${text}`);
        setError({ friendly: t(key), raw: text });
        return;
      }
      const body = await r.json();
      // Handle dedup match block (spec-deduplication).
      if (body.spec_match) {
        // MatchDialog is rendered in the parent (alfred/page.tsx) via specMatch state.
        window.dispatchEvent(new CustomEvent("alfred:spec_match", { detail: body.spec_match }));
        setActiveCard(null);
        return;
      }
      if (body.draft?.draft_id) setDraftId(body.draft.draft_id);
      if (body.next_question) {
        setMessages([{ role: "assistant", content: body.next_question }]);
      }
      // sdlc-end-to-end (10.1): intent committed immediately on start
      if (body.openspec_id && body.app_id && activeCard === "new_app") {
        setCommittedIntent({ openspec_id: body.openspec_id, app_id: body.app_id });
      }
    } catch (err) {
      setError({ friendly: t("alfred_err_generic"), raw: String(err) });
    } finally {
      setLoading(false);
    }
  }

  async function sendMessage() {
    if (!input.trim() || !draftId) return;
    const userText = input.trim();
    setInput("");
    setMessages((prev) => [...prev, { role: "user", content: userText }]);
    setLoading(true);
    setError(null);

    try {
      const r = await fetch(`/api/alfred/intent/answer`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ draft_id: draftId, answer: userText, view: "friendly" }),
      });
      if (!r.ok) {
        const text = await r.text();
        const key = mapErrorCode(`${r.status} ${text}`);
        setError({ friendly: t(key), raw: text });
        return;
      }
      const body = await r.json();
      if (body.next_question) {
        setMessages((prev) => [...prev, { role: "assistant", content: body.next_question }]);
      }
      // sdlc-end-to-end (10.1): capture committed intent data when conversation ends
      if (!body.next_question) {
        if (body.openspec_id && body.app_id && activeCard === "new_app") {
          setCommittedIntent({ openspec_id: body.openspec_id, app_id: body.app_id });
        } else {
          setMessages((prev) => [
            ...prev,
            { role: "assistant", content: t("alfred_card_start") + "." },
          ]);
        }
      }
    } catch (err) {
      setError({ friendly: t("alfred_err_generic"), raw: String(err) });
    } finally {
      setLoading(false);
    }
  }

  // Landing: three cards
  if (!activeCard) {
    return (
      <div className="alfred-friendly-root">
        <header className="alfred-friendly-header">
          <h1 className="page-title">
            {t("alfred_friendly_title")} <em>Console</em>
          </h1>
          <p className="page-sub">{t("alfred_friendly_subtitle")}</p>
          <button
            type="button"
            className="alfred-friendly-switch text-sm"
            onClick={onSwitchView}
            style={{ color: "var(--fg-3)", marginTop: 8 }}
          >
            {t("alfred_friendly_switch_dev")} →
          </button>
        </header>

        <div className="alfred-cards-grid">
          {cards.map((card) => (
            <div
              key={card.kind}
              className="alfred-card"
              role="button"
              tabIndex={0}
              onClick={() => startCard(card.kind)}
              onKeyDown={(e) => e.key === "Enter" && startCard(card.kind)}
            >
              <h2 className="alfred-card-title">
                <em>{card.title}</em>
              </h2>
              <p className="alfred-card-body">{card.body}</p>
              <span className="alfred-card-cta">{t("alfred_card_start")}</span>
            </div>
          ))}
        </div>

        {/* App switcher shown when > 1 visible App */}
        {apps.filter((a) => !a.slug.startsWith("_")).length > 0 && (
          <div className="alfred-friendly-app-switcher">
            <AppSwitcher
              apps={apps}
              activeAppId={activeAppId}
              onSwitch={(id) => setActiveAppId(id)}
            />
          </div>
        )}
      </div>
    );
  }

  // Conversation panel
  return (
    <div className="alfred-friendly-root">
      <div className="alfred-conversation-header">
        <button
          type="button"
          className="icon-btn"
          onClick={() => { setActiveCard(null); setMessages([]); }}
          aria-label={t("alfred_friendly_back")}
        >
          ←
        </button>
        <AppSwitcher
          apps={apps}
          activeAppId={activeAppId}
          onSwitch={(id) => setActiveAppId(id)}
        />
        <button
          type="button"
          className="text-sm"
          onClick={onSwitchView}
          style={{ marginLeft: "auto", color: "var(--fg-3)" }}
        >
          {t("alfred_friendly_switch_dev")}
        </button>
      </div>

      {error && (
        <div className="alfred-error-block" role="alert">
          <p>{error.friendly}</p>
          <button
            type="button"
            onClick={() => setShowRaw((s) => !s)}
            className="text-xs"
            style={{ color: "var(--fg-3)" }}
          >
            {showRaw ? t("alfred_err_hide_details") : t("alfred_err_show_details")}
          </button>
          {showRaw && (
            <pre className="text-xs mt-2 rounded p-2 bg-neutral-100 dark:bg-neutral-900 overflow-x-auto">
              {error.raw}
            </pre>
          )}
        </div>
      )}

      <div className="alfred-transcript" aria-live="polite">
        {messages.map((msg, i) => (
          <div
            key={i}
            className={`alfred-msg alfred-msg--${msg.role}`}
          >
            <div className="alfred-msg-bubble">{msg.content}</div>
          </div>
        ))}
        {loading && (
          <div className="alfred-msg alfred-msg--assistant">
            <div className="alfred-msg-bubble alfred-msg-bubble--loading">…</div>
          </div>
        )}
      </div>

      {/* sdlc-end-to-end (10.1): workflow trigger after intent committed for new_app */}
      {activeCard === "new_app" && committedIntent && !workflowStarted && (
        <div className="alfred-workflow-trigger" style={{ padding: "12px 16px", borderTop: "1px solid var(--border-1)" }}>
          <button
            type="button"
            className="rounded bg-neutral-900 px-4 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900"
            disabled={loading}
            onClick={() => void launchIntentToInfrastructure(committedIntent.openspec_id, committedIntent.app_id)}
          >
            {t("alfred_new_app_run_workflow")}
          </button>
        </div>
      )}

      {/* sdlc-end-to-end (10.2): healing L1/L2 events panel for Operar card */}
      {activeCard === "operate_app" && (
        <div className="alfred-healing-panel" style={{ padding: "12px 16px", borderTop: "1px solid var(--border-1)" }}>
          <h3 className="text-sm font-semibold mb-2" style={{ color: "var(--fg-1)" }}>
            {t("alfred_operate_healing_title")}
          </h3>
          {healingLoading && (
            <p className="text-xs" style={{ color: "var(--fg-3)" }}>{t("alfred_operate_healing_loading")}</p>
          )}
          {healingError && (
            <p className="text-xs" style={{ color: "var(--fg-danger)" }}>{t("alfred_operate_healing_err")}</p>
          )}
          {!healingLoading && !healingError && healingEvents.length === 0 && (
            <p className="text-xs" style={{ color: "var(--fg-3)" }}>{t("alfred_operate_healing_empty")}</p>
          )}
          {!healingLoading && !healingError && healingEvents.map((ev) => (
            <div key={ev.id} className="alfred-healing-event" style={{ marginBottom: 8, padding: "8px 10px", borderRadius: 6, background: "var(--bg-2)" }}>
              <div className="text-xs font-medium" style={{ color: "var(--fg-1)" }}>
                {ev.type === "l1" ? t("alfred_operate_healing_l1") : t("alfred_operate_healing_l2")}
                {" · "}
                <span style={{ color: "var(--fg-3)" }}>{t("alfred_operate_healing_severity").replace("{severity}", ev.severity)}</span>
              </div>
              <p className="text-xs mt-1" style={{ color: "var(--fg-2)" }}>{ev.summary}</p>
            </div>
          ))}
        </div>
      )}

      {draftId && (
        <form
          className="alfred-input-row"
          onSubmit={(e) => { e.preventDefault(); void sendMessage(); }}
        >
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={t("alfred_friendly_placeholder")}
            className="alfred-input"
            disabled={loading}
            aria-label={t("alfred_friendly_placeholder")}
          />
          <button
            type="submit"
            className="rounded bg-neutral-900 px-3 py-2 text-sm font-medium text-white dark:bg-neutral-100 dark:text-neutral-900"
            disabled={loading || !input.trim()}
          >
            {t("alfred_friendly_send")}
          </button>
        </form>
      )}
    </div>
  );
}
