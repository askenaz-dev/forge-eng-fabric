"use client";

import type { OnboardingEvent } from "@/lib/onboarding-types";
import { useEffect, useState } from "react";

export function LiveEvents({ requestId, initialEvents, initialStatus }: { requestId: string; initialEvents: OnboardingEvent[]; initialStatus: string }) {
  const [events, setEvents] = useState(initialEvents);
  const [status, setStatus] = useState(initialStatus);
  const terminal = status === "completed" || status === "failed";

  useEffect(() => {
    if (terminal) return;
    const timer = window.setInterval(async () => {
      const [timelineResponse, requestResponse] = await Promise.all([
        fetch(`/api/onboarding/${requestId}/timeline`, { cache: "no-store" }),
        fetch(`/api/onboarding/${requestId}`, { cache: "no-store" }),
      ]);
      if (timelineResponse.ok) {
        const payload = (await timelineResponse.json()) as { events: OnboardingEvent[] };
        setEvents(payload.events);
      }
      if (requestResponse.ok) {
        const payload = (await requestResponse.json()) as { status: string };
        setStatus(payload.status);
      }
    }, 2000);
    return () => window.clearInterval(timer);
  }, [requestId, terminal]);

  return (
    <div className="rounded-3xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-xl font-semibold">Live events</h3>
          <p className="mt-1 text-sm opacity-70">Polling timeline events from the onboarding service until terminal state.</p>
        </div>
        <span className="rounded bg-neutral-100 px-2 py-1 text-xs font-semibold dark:bg-neutral-800">{status}</span>
      </div>
      <ol className="mt-5 grid gap-3">
        {events.map((event) => (
          <li key={event.id} className="grid gap-2 rounded-2xl border border-neutral-200 p-4 dark:border-neutral-800 md:grid-cols-[180px_1fr_120px]">
            <span className="text-sm font-medium">{event.stage}</span>
            <span className="text-sm opacity-70">{event.message || JSON.stringify(event.payload ?? {})}</span>
            <span className="text-right text-xs uppercase tracking-wide opacity-60">{event.outcome} {event.duration_ms ? `${event.duration_ms}ms` : ""}</span>
          </li>
        ))}
        {events.length === 0 && <li className="rounded border border-dashed border-neutral-300 p-6 text-sm opacity-70 dark:border-neutral-800">No events have been emitted yet.</li>}
      </ol>
    </div>
  );
}
