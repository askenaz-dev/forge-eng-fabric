import { Card } from "./Card";
import type { ReactNode } from "react";

// UpstreamError renders a friendly, actionable message when a backend call
// fails. It distinguishes auth (401), forbidden (403), unreachable (5xx /
// network) and unknown errors so the user sees what to fix instead of a raw
// "service 401" toast.
export type UpstreamErrorKind = "auth" | "forbidden" | "unreachable" | "unknown";

export type UpstreamErrorProps = {
  service: string;
  status?: number;
  message?: string;
  hint?: ReactNode;
};

export function classifyUpstreamError(message: string | undefined | null): { kind: UpstreamErrorKind; status?: number } {
  if (!message) return { kind: "unknown" };
  const m = /(\b)(\d{3})(\b)/.exec(message);
  const status = m ? Number(m[2]) : undefined;
  if (status === 401) return { kind: "auth", status };
  if (status === 403) return { kind: "forbidden", status };
  if (status && status >= 500) return { kind: "unreachable", status };
  if (/unreachable|ECONNREFUSED|ENOTFOUND|fetch failed|network/i.test(message)) {
    return { kind: "unreachable", status };
  }
  return { kind: "unknown", status };
}

export function UpstreamError({ service, status, message, hint }: UpstreamErrorProps) {
  const { kind } = classifyUpstreamError(message ?? (status ? `${service} ${status}` : undefined));
  const heading =
    kind === "auth"        ? `${service} requires authentication`
  : kind === "forbidden"   ? `${service} refused this user`
  : kind === "unreachable" ? `${service} is unreachable`
  :                          `${service} returned an error`;

  const explainer =
    kind === "auth"
      ? "The portal called the service but the bearer token was missing or invalid. Common causes: Keycloak is not running, your session expired, or the service rejected the audience."
      : kind === "forbidden"
      ? "Your user is authenticated but lacks the role required for this action."
      : kind === "unreachable"
      ? "The service did not respond. It might not be running locally, or its network endpoint is wrong."
      : "Unexpected upstream failure. Check the service logs for the correlation id.";

  return (
    <Card style={{ marginBottom: 16 }}>
      <div style={{ padding: 14 }}>
        <p style={{ margin: 0, fontWeight: 500 }}>{heading}{status ? ` (HTTP ${status})` : ""}</p>
        <p style={{ margin: "6px 0 0", color: "var(--fg-2)", fontSize: 13 }}>{explainer}</p>
        {hint && <div style={{ marginTop: 10, fontSize: 13 }}>{hint}</div>}
        {message && (
          <p style={{ margin: "10px 0 0", fontFamily: "var(--f-mono)", fontSize: 11, color: "var(--fg-3)" }}>
            {message}
          </p>
        )}
      </div>
    </Card>
  );
}
