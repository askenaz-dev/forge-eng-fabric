"use client";

import { useEffect } from "react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("[portal] root layout error", error);
  }, [error]);

  return (
    <html lang="es">
      <body
        style={{
          margin: 0,
          fontFamily:
            'ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif',
          background: "#FAFAF7",
          color: "#13110F",
        }}
      >
        <div style={{ maxWidth: 640, padding: 32, margin: "48px auto" }}>
          <h1 style={{ fontSize: 28, marginBottom: 12 }}>Forge no pudo arrancar.</h1>
          <p style={{ color: "#57544F", marginBottom: 16, lineHeight: 1.5 }}>
            El layout raíz lanzó un error antes de poder renderizar el portal. Esto
            suele ser un problema de configuración (Keycloak, servicios upstream o
            variables de entorno).
          </p>
          {error?.digest && (
            <code
              style={{
                display: "inline-block",
                fontSize: 12,
                color: "#908A80",
                marginBottom: 16,
              }}
            >
              digest: {error.digest}
            </code>
          )}
          <pre
            style={{
              background: "#F2EFE8",
              border: "1px solid #E5E0D2",
              borderRadius: 8,
              padding: 12,
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
              border: "1px solid #BBB2A0",
              background: "#FFFFFF",
              padding: "8px 14px",
              borderRadius: 6,
              fontSize: 13,
              cursor: "pointer",
            }}
          >
            Reintentar
          </button>
        </div>
      </body>
    </html>
  );
}
