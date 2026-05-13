"use client";

import { useEffect, useRef } from "react";

export type SSEEvent = {
  id?: string;
  type: string;
  payload?: Record<string, unknown>;
};

// A tiny event bus for the dashboard panels to subscribe to a single shared
// SSE connection. Each panel can subscribe to specific event types and
// receive a typed callback. Connection is shared across all subscribers.

type Handler = (event: SSEEvent) => void;

let connection: EventSource | null = null;
let handlers: Set<Handler> = new Set();
let reconnectTimer: ReturnType<typeof setTimeout> | undefined;

function connect() {
  if (connection || typeof window === "undefined") return;
  try {
    connection = new EventSource("/api/notifications/stream");
    connection.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data) as SSEEvent;
        for (const h of handlers) h(data);
      } catch {
        // ignore malformed payloads
      }
    };
    connection.onerror = () => {
      connection?.close();
      connection = null;
      if (!reconnectTimer) {
        reconnectTimer = setTimeout(() => {
          reconnectTimer = undefined;
          if (handlers.size > 0) connect();
        }, 5000);
      }
    };
  } catch {
    // SSE may be blocked in some browsers; we degrade silently.
  }
}

function subscribe(handler: Handler): () => void {
  handlers.add(handler);
  if (!connection) connect();
  return () => {
    handlers.delete(handler);
    if (handlers.size === 0) {
      connection?.close();
      connection = null;
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = undefined;
      }
    }
  };
}

export function useSSE(eventTypes: string[], onEvent: (event: SSEEvent) => void): void {
  const handlerRef = useRef(onEvent);
  handlerRef.current = onEvent;
  const key = eventTypes.join("|");
  useEffect(() => {
    const types = new Set(key.split("|").filter(Boolean));
    const unsubscribe = subscribe((event) => {
      if (types.size === 0 || matchesAny(event.type, types)) {
        handlerRef.current(event);
      }
    });
    return unsubscribe;
  }, [key]);
}

function matchesAny(type: string, patterns: Set<string>): boolean {
  if (patterns.has(type)) return true;
  for (const p of patterns) {
    if (p.endsWith("*") && type.startsWith(p.slice(0, -1))) return true;
  }
  return false;
}
