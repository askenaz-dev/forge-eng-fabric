/**
 * Resolves entity IDs to human-readable labels for the Friendly view.
 * (alfred-console-redesign requirement 1.4)
 *
 * The resolver maintains a small in-memory cache populated from the API.
 * On lookup failure it returns the `fallback` italic placeholder and logs to
 * the portal telemetry stream.
 */

type EntityKind = "app" | "spec" | "workspace" | "run";

interface LabelEntry {
  label: string;
  fetchedAt: number;
}

const cache = new Map<string, LabelEntry>();
const TTL_MS = 60_000;

function cacheKey(kind: EntityKind, id: string) {
  return `${kind}:${id}`;
}

function getCached(kind: EntityKind, id: string): string | null {
  const entry = cache.get(cacheKey(kind, id));
  if (!entry) return null;
  if (Date.now() - entry.fetchedAt > TTL_MS) {
    cache.delete(cacheKey(kind, id));
    return null;
  }
  return entry.label;
}

function setCached(kind: EntityKind, id: string, label: string) {
  cache.set(cacheKey(kind, id), { label, fetchedAt: Date.now() });
}

/** Resolve an entity label, returning `fallback` on any failure. */
export async function resolveLabel(
  kind: EntityKind,
  id: string,
  fallback: string,
): Promise<string> {
  if (!id || id === "_unassigned") return fallback;
  const hit = getCached(kind, id);
  if (hit) return hit;

  try {
    const endpoints: Record<EntityKind, string> = {
      app: `/api/apps/${id}`,
      spec: `/api/openspecs/${id}`,
      workspace: `/api/workspaces/${id}`,
      run: `/api/runs/${id}`,
    };
    const r = await fetch(endpoints[kind], { cache: "no-store" });
    if (!r.ok) throw new Error(`label ${r.status}`);
    const body = await r.json();
    const label: string = body.name ?? body.title ?? body.slug ?? id;
    setCached(kind, id, label);
    return label;
  } catch (err) {
    try {
      await fetch("/api/alfred/telemetry", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ event: "label_resolve_failed", kind, id, error: String(err) }),
        keepalive: true,
      });
    } catch {
      // best-effort telemetry
    }
    return fallback;
  }
}

/**
 * Replace raw entity IDs in Alfred-authored text with human labels.
 * Matches patterns like `app-1`, `app:app-1`, `spec-7b3`.
 * Used by the Friendly view transcript renderer (requirement 1.4).
 */
export async function resolveTextLabels(
  text: string,
  appLabels: Record<string, string>,
): Promise<string> {
  let out = text;
  for (const [id, label] of Object.entries(appLabels)) {
    out = out.replaceAll(id, label);
  }
  return out;
}
