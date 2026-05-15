"use client";

// Shows for mirrored public-origin asset versions.
// Props: assetId, version, originRef, lastSyncedAt, autoPromotePolicy, onAction

import { useState } from "react";
import { useRouter } from "next/navigation";

type AutoPromotePolicy = "none" | "patch" | "minor" | "major";

export type MirroredVersionCardProps = {
  assetId: string;
  version: string;
  originRef: string;
  lastSyncedAt: string | null;
  autoPromotePolicy: AutoPromotePolicy;
  /** Called after a successful promote/reject transition. */
  onAction?: (action: "approved" | "rejected") => void;
};

/**
 * Derives the version bump kind between two semver strings.
 * Returns "patch", "minor", "major", or null if parsing fails.
 */
function bumpKind(current: string, upstream: string): "patch" | "minor" | "major" | null {
  const parse = (v: string) => {
    const m = v.replace(/^[^0-9]*/, "").match(/^(\d+)\.(\d+)\.(\d+)/);
    if (!m) return null;
    return { major: Number(m[1]), minor: Number(m[2]), patch: Number(m[3]) };
  };
  const c = parse(current);
  const u = parse(upstream);
  if (!c || !u) return null;
  if (u.major !== c.major) return "major";
  if (u.minor !== c.minor) return "minor";
  return "patch";
}

/**
 * Parses the upstream version out of an originRef string like
 * `npm:my-skill@1.2.3` and returns it. Falls back to the full ref.
 */
function upstreamVersion(originRef: string): string {
  const m = originRef.match(/@([^@]+)$/);
  return m ? m[1] : originRef;
}

/** Formats an ISO date string as relative time (e.g. "3 hours ago"). */
function relativeTime(iso: string | null): string {
  if (!iso) return "unknown";
  const diffMs = Date.now() - new Date(iso).getTime();
  if (Number.isNaN(diffMs)) return iso;
  const diffSec = Math.floor(diffMs / 1000);
  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  return `${diffDay}d ago`;
}

export function MirroredVersionCard({
  assetId,
  version,
  originRef,
  lastSyncedAt,
  autoPromotePolicy,
  onAction,
}: MirroredVersionCardProps) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const upstream = upstreamVersion(originRef);
  const bump = bumpKind(version, upstream);
  const autoPolicyActive =
    autoPromotePolicy !== "none" &&
    bump !== null &&
    (autoPromotePolicy === "minor"
      ? bump === "patch" || bump === "minor"
      : autoPromotePolicy === "patch"
      ? bump === "patch"
      : true /* "major" covers all */);

  async function transition(to: "approved" | "rejected") {
    setBusy(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/assets/${encodeURIComponent(assetId)}/versions/${encodeURIComponent(version)}/transition`,
        {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ to }),
        },
      );
      const payload = (await res.json().catch(() => ({}))) as { error?: string; message?: string };
      if (!res.ok) throw new Error(payload.error ?? payload.message ?? `transition ${res.status}`);
      onAction?.(to);
      router.refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "action failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <article
      data-testid="mirrored-version-card"
      style={{
        border: "1px solid var(--border-1, #e2e8f0)",
        borderRadius: 8,
        padding: "16px 20px",
        background: "var(--bg-2, #f8fafc)",
        display: "flex",
        flexDirection: "column",
        gap: 12,
      }}
    >
      <header style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span
          style={{
            fontSize: 10,
            fontWeight: 700,
            letterSpacing: "0.08em",
            textTransform: "uppercase",
            background: "#dbeafe",
            color: "#1e40af",
            borderRadius: 4,
            padding: "2px 6px",
          }}
        >
          Mirrored version
        </span>
        <span style={{ fontSize: 13, color: "var(--fg-1, #1e293b)", fontFamily: "var(--f-mono, monospace)" }}>
          {originRef}
        </span>
      </header>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 12 }}>
        <InfoCell label="Current version" value={version} mono />
        <InfoCell label="Upstream version" value={upstream} mono highlight={version !== upstream} />
        {bump && <InfoCell label="Bump kind" value={bump} />}
      </div>

      <p style={{ fontSize: 12, color: "var(--fg-1, #64748b)", margin: 0 }}>
        Last synced:{" "}
        <span title={lastSyncedAt ?? ""}>{relativeTime(lastSyncedAt)}</span>
      </p>

      {error && (
        <p
          style={{
            fontSize: 13,
            color: "#991b1b",
            border: "1px solid #fca5a5",
            background: "#fef2f2",
            borderRadius: 6,
            padding: "8px 12px",
            margin: 0,
          }}
        >
          {error}
        </p>
      )}

      {autoPolicyActive ? (
        <AutoPromoteNotice policy={autoPromotePolicy} bump={bump} />
      ) : (
        <div style={{ display: "flex", gap: 8 }}>
          <button
            data-testid="mirrored-promote-btn"
            disabled={busy}
            onClick={() => transition("approved")}
            style={{
              padding: "6px 16px",
              borderRadius: 6,
              border: "none",
              background: "#166534",
              color: "#fff",
              fontWeight: 600,
              fontSize: 13,
              cursor: busy ? "not-allowed" : "pointer",
              opacity: busy ? 0.6 : 1,
            }}
          >
            {busy ? "…" : "Promote"}
          </button>
          <button
            data-testid="mirrored-reject-btn"
            disabled={busy}
            onClick={() => transition("rejected")}
            style={{
              padding: "6px 16px",
              borderRadius: 6,
              border: "1px solid var(--border-1, #e2e8f0)",
              background: "transparent",
              color: "#991b1b",
              fontWeight: 600,
              fontSize: 13,
              cursor: busy ? "not-allowed" : "pointer",
              opacity: busy ? 0.6 : 1,
            }}
          >
            Reject
          </button>
        </div>
      )}
    </article>
  );
}

function InfoCell({
  label,
  value,
  mono,
  highlight,
}: {
  label: string;
  value: string;
  mono?: boolean;
  highlight?: boolean;
}) {
  return (
    <div
      style={{
        border: "1px solid var(--border-1, #e2e8f0)",
        borderRadius: 6,
        padding: "8px 12px",
        background: highlight ? "#fefce8" : "transparent",
      }}
    >
      <p style={{ fontSize: 10, textTransform: "uppercase", letterSpacing: "0.06em", color: "#64748b", margin: "0 0 4px" }}>
        {label}
      </p>
      <p style={{ fontSize: 13, fontFamily: mono ? "var(--f-mono, monospace)" : "inherit", margin: 0, fontWeight: 600 }}>
        {value}
      </p>
    </div>
  );
}

function AutoPromoteNotice({
  policy,
  bump,
}: {
  policy: AutoPromotePolicy;
  bump: "patch" | "minor" | "major" | null;
}) {
  return (
    <p
      data-testid="auto-promote-notice"
      style={{
        fontSize: 13,
        color: "#1e40af",
        border: "1px solid #bfdbfe",
        background: "#eff6ff",
        borderRadius: 6,
        padding: "8px 12px",
        margin: 0,
      }}
    >
      Auto-promote is configured (policy: <strong>{policy}</strong>
      {bump ? `, detected bump: ${bump}` : ""}). This version will be promoted automatically.
    </p>
  );
}
