"use client";

import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

export type AlfredStepStatus =
  | "pending"
  | "running"
  | "paused_for_approval"
  | "paused_for_budget"
  | "succeeded"
  | "failed"
  | "skipped"
  | "cancelled";

export type AlfredSessionStatus =
  | "planning"
  | "running"
  | "paused_for_approval"
  | "paused_for_budget"
  | "completed"
  | "aborted"
  | "failed";

export type AlfredStepEvent = {
  idx: number;
  kind: string;
  status: AlfredStepStatus;
  summary?: string;
};

export type AlfredSessionEvent = {
  status: AlfredSessionStatus;
  plan_revision: number;
};

export type AlfredTranscriptEntry = AlfredStepEvent | AlfredSessionEvent;

type AlfredSessionContextValue = {
  open: boolean;
  show: () => void;
  hide: () => void;
  toggle: () => void;
  activeSessionId: string | null;
  status: AlfredSessionStatus | null;
  steps: AlfredStepEvent[];
  start: (input: { workspaceId: string; openspecId: string; intent: string }) => Promise<string | null>;
  sendFollowUp: (intent: string) => Promise<void>;
  cancel: () => Promise<void>;
};

const AlfredSessionContext = createContext<AlfredSessionContextValue | null>(null);

export function AlfredSessionProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState(false);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [status, setStatus] = useState<AlfredSessionStatus | null>(null);
  const [steps, setSteps] = useState<AlfredStepEvent[]>([]);
  const sseRef = useRef<EventSource | null>(null);

  const show = useCallback(() => {
    setOpen(true);
    void emitTelemetry("portal.alfred.dock_opened.v1", { session_id: activeSessionId });
  }, [activeSessionId]);
  const hide = useCallback(() => {
    setOpen(false);
    void emitTelemetry("portal.alfred.dock_closed.v1", { session_id: activeSessionId });
  }, [activeSessionId]);
  const toggle = useCallback(() => {
    setOpen((o) => {
      const next = !o;
      void emitTelemetry(
        next ? "portal.alfred.dock_opened.v1" : "portal.alfred.dock_closed.v1",
        { session_id: activeSessionId },
      );
      return next;
    });
  }, [activeSessionId]);

  // Subscribe to SSE for the active session.
  useEffect(() => {
    if (!activeSessionId) return;
    const url = `/api/alfred/sessions/${activeSessionId}/stream`;
    const es = new EventSource(url);
    sseRef.current = es;
    es.addEventListener("step", (e) => {
      try {
        const payload = JSON.parse((e as MessageEvent).data) as AlfredStepEvent;
        setSteps((prev) => {
          const existing = prev.find((s) => s.idx === payload.idx);
          if (existing) return prev.map((s) => (s.idx === payload.idx ? { ...s, ...payload } : s));
          return [...prev, payload];
        });
      } catch {
        // ignored
      }
    });
    es.addEventListener("session", (e) => {
      try {
        const payload = JSON.parse((e as MessageEvent).data) as AlfredSessionEvent;
        setStatus(payload.status);
      } catch {
        // ignored
      }
    });
    es.onerror = () => {
      // Browser auto-reconnects via Last-Event-ID; nothing to do.
    };
    return () => {
      es.close();
      sseRef.current = null;
    };
  }, [activeSessionId]);

  const start = useCallback<AlfredSessionContextValue["start"]>(async (input) => {
    const r = await fetch("/api/alfred/sessions", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        workspace_id: input.workspaceId,
        openspec_id: input.openspecId,
        intent: input.intent,
      }),
    });
    if (!r.ok) return null;
    const body = (await r.json()) as { session_id?: string; status?: AlfredSessionStatus };
    if (!body.session_id) return null;
    setActiveSessionId(body.session_id);
    setStatus(body.status ?? "planning");
    setSteps([]);
    setOpen(true);
    void emitTelemetry("portal.alfred.dock_session_started.v1", {
      session_id: body.session_id,
      openspec_id: input.openspecId,
    });
    return body.session_id;
  }, []);

  const sendFollowUp = useCallback<AlfredSessionContextValue["sendFollowUp"]>(
    async (intent) => {
      if (!activeSessionId) return;
      await fetch(`/api/alfred/sessions/${activeSessionId}/messages`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ intent }),
      });
      void emitTelemetry("portal.alfred.dock_follow_up_sent.v1", {
        session_id: activeSessionId,
      });
    },
    [activeSessionId],
  );

  const cancel = useCallback<AlfredSessionContextValue["cancel"]>(async () => {
    if (!activeSessionId) return;
    await fetch(`/api/alfred/sessions/${activeSessionId}/cancel`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ reason: "cancelled from dock" }),
    });
  }, [activeSessionId]);

  const value = useMemo<AlfredSessionContextValue>(
    () => ({
      open,
      show,
      hide,
      toggle,
      activeSessionId,
      status,
      steps,
      start,
      sendFollowUp,
      cancel,
    }),
    [open, show, hide, toggle, activeSessionId, status, steps, start, sendFollowUp, cancel],
  );

  return <AlfredSessionContext.Provider value={value}>{children}</AlfredSessionContext.Provider>;
}

export function useAlfredSession(): AlfredSessionContextValue {
  const ctx = useContext(AlfredSessionContext);
  if (!ctx) throw new Error("useAlfredSession must be used inside <AlfredSessionProvider>");
  return ctx;
}

export function recordArtifactNavigation(href: string) {
  void emitTelemetry("portal.alfred.dock_navigated_to_artifact.v1", { href });
}

async function emitTelemetry(type: string, data: Record<string, unknown>) {
  try {
    await fetch("/api/alfred/telemetry", {
      method: "POST",
      headers: { "content-type": "application/json" },
      keepalive: true,
      body: JSON.stringify({ type, data }),
    });
  } catch {
    // best-effort
  }
}
