export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    const { registerOTel } = await import("@vercel/otel");
    registerOTel({
      serviceName: "portal",
      attributes: {
        "deployment.environment": process.env.ENV ?? "local",
      },
    });
  }
}

// OpenTelemetry-style instrumentation spans for the Portal.
//
// Conventions:
//   - "portal.dashboard.<panel>"       Dashboard data-loading spans
//   - "portal.shell.<action>"          Shell mutations (theme, lang, density, workspace)
//   - "portal.command-palette.<phase>" Palette aggregation, audit
//   - "portal.api.<route>"             Each Route Handler under /api/*
//
// `traceSpan` wraps an async function with a span tagged with the canonical
// name + attributes. We use the OTel global tracer if available; otherwise
// the function executes without instrumentation.

import { trace, type SpanStatusCode } from "@opentelemetry/api";

export function tracer() {
  return trace.getTracer("@forge/portal");
}

export async function traceSpan<T>(
  name: string,
  attributes: Record<string, string | number | boolean | undefined>,
  fn: () => Promise<T>,
): Promise<T> {
  const span = tracer().startSpan(name);
  for (const [k, v] of Object.entries(attributes)) {
    if (v != null) span.setAttribute(k, v);
  }
  try {
    const value = await fn();
    span.setStatus({ code: 1 as unknown as SpanStatusCode });
    return value;
  } catch (err) {
    span.recordException(err as Error);
    span.setStatus({ code: 2 as unknown as SpanStatusCode, message: (err as Error).message });
    throw err;
  } finally {
    span.end();
  }
}
