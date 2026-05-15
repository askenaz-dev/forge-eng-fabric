// Renders a small "PUBLIC ORIGIN" chip badge.
// Props: originRef, lastSyncedAt (optional)
// Only renders when is_public_origin === true.
// Tooltip (title attr) shows originRef + last_synced_at.

export type PublicOriginBadgeProps = {
  isPublicOrigin: boolean;
  originRef?: string | null;
  lastSyncedAt?: string | null;
};

/**
 * A small inline badge that identifies an asset as having a public origin.
 * Renders nothing unless `isPublicOrigin` is true.
 */
export function PublicOriginBadge({ isPublicOrigin, originRef, lastSyncedAt }: PublicOriginBadgeProps) {
  if (!isPublicOrigin) return null;

  const titleParts: string[] = [];
  if (originRef) titleParts.push(originRef);
  if (lastSyncedAt) titleParts.push(`synced ${lastSyncedAt}`);
  const tooltipText = titleParts.join(" · ");

  return (
    <span
      data-testid="public-origin-badge"
      title={tooltipText || undefined}
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: 4,
        fontSize: 10,
        fontWeight: 700,
        letterSpacing: "0.08em",
        textTransform: "uppercase",
        color: "var(--fg-1, #1e293b)",
        background: "var(--bg-2, #f1f5f9)",
        border: "1px solid var(--border-1, #e2e8f0)",
        borderRadius: 4,
        padding: "2px 6px",
        whiteSpace: "nowrap",
        userSelect: "none",
      }}
    >
      {/* globe icon via CSS — no external dependency needed */}
      <svg
        aria-hidden="true"
        width="10"
        height="10"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="8" cy="8" r="7" />
        <path d="M8 1c-2 3-2 11 0 14M8 1c2 3 2 11 0 14M1 8h14" />
      </svg>
      Public origin
    </span>
  );
}
