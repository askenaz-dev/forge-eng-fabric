"use client";

import { useEffect } from "react";

export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("[portal] runtime error", error);
  }, [error]);

  return (
    <div
      role="alert"
      style={{
        padding: 32,
        maxWidth: 720,
        margin: "48px auto",
        fontFamily: "var(--f-sans, system-ui)",
        color: "var(--fg, #13110F)",
      }}
    >
      <h1
        style={{
          fontFamily: "var(--f-display, Georgia, serif)",
          fontStyle: "italic",
          fontSize: 32,
          marginBottom: 12,
        }}
      >
        Algo se rompió en el portal.
      </h1>
      <p style={{ color: "var(--fg-2, #57544F)", marginBottom: 16, lineHeight: 1.5 }}>
        El servidor devolvió un error mientras renderizaba esta página. Puedes intentar de
        nuevo; si persiste, copia el detalle y compártelo con el equipo de plataforma.
      </p>
      {error?.digest && (
        <code
          style={{
            display: "inline-block",
            fontFamily: "var(--f-mono, ui-monospace)",
            fontSize: 12,
            color: "var(--fg-3, #908A80)",
            marginBottom: 16,
          }}
        >
          digest: {error.digest}
        </code>
      )}
      <pre
        style={{
          background: "var(--bg-sunk, #F2EFE8)",
          border: "1px solid var(--border, #E5E0D2)",
          borderRadius: 8,
          padding: 12,
          fontFamily: "var(--f-mono, ui-monospace)",
          fontSize: 12,
          whiteSpace: "pre-wrap",
          wordBreak: "break-word",
          marginBottom: 16,
        }}
      >
        {error?.message || "Internal error"}
      </pre>
      <button
        type="button"
        onClick={() => reset()}
        style={{
          appearance: "none",
          border: "1px solid var(--border-strong, #BBB2A0)",
          background: "var(--bg-card, #FFFFFF)",
          color: "var(--fg, #13110F)",
          padding: "8px 14px",
          borderRadius: 6,
          fontSize: 13,
          cursor: "pointer",
        }}
      >
        Reintentar
      </button>
    </div>
  );
}
