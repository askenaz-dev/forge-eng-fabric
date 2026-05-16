"use client";

// Client-side form for registering a custom node manifest URL + endpoint.
// Fetches the manifest, validates it with the SDK's validateManifest, and
// POSTs the registration to /api/admin/custom-nodes for persistence.

import { useState } from "react";
import { validateManifest } from "@/components/flow/customNodeApi";

export function CustomNodeForm() {
  const [manifestURL, setManifestURL] = useState("");
  const [endpoint, setEndpoint] = useState("");
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<string[] | null>(null);
  const [ok, setOk] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErrors(null);
    setOk(null);
    try {
      const r = await fetch(manifestURL);
      if (!r.ok) {
        setErrors([`manifest fetch ${r.status}`]);
        return;
      }
      const text = await r.text();
      // Manifest is YAML but JSON is also accepted; for v0 we just attempt JSON
      // and fall back to a parse error if it's YAML — production parses both.
      let parsed: unknown;
      try {
        parsed = JSON.parse(text);
      } catch {
        setErrors(["manifest_not_json: JSON-encoded manifests only in v0 of the admin form (YAML coming)"]);
        return;
      }
      const errs = validateManifest(parsed);
      if (errs) {
        setErrors(errs);
        return;
      }
      const persist = await fetch("/api/admin/custom-nodes", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ manifest: parsed, manifest_url: manifestURL, endpoint }),
      });
      if (!persist.ok) {
        setErrors([`persist failed ${persist.status}: ${await persist.text()}`]);
        return;
      }
      setOk("Registered. The node now appears in this workspace's palette under Custom.");
    } catch (err) {
      setErrors([err instanceof Error ? err.message : String(err)]);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={submit} className="max-w-xl space-y-3 text-sm">
      <label className="block">
        <span className="block text-xs uppercase tracking-wide opacity-60 mb-1">Manifest URL</span>
        <input
          type="url"
          value={manifestURL}
          onChange={(e) => setManifestURL(e.target.value)}
          placeholder="https://nodes.example.com/uppercase-text/manifest.json"
          required
          className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
        />
      </label>
      <label className="block">
        <span className="block text-xs uppercase tracking-wide opacity-60 mb-1">Endpoint URL</span>
        <input
          type="url"
          value={endpoint}
          onChange={(e) => setEndpoint(e.target.value)}
          placeholder="https://nodes.example.com/uppercase-text/invoke"
          required
          className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
        />
      </label>
      {errors && (
        <ul role="alert" className="rounded border border-rose-300 bg-rose-50 p-2 text-xs text-rose-800 dark:border-rose-700 dark:bg-rose-950 dark:text-rose-200">
          {errors.map((e, i) => (
            <li key={i}>{e}</li>
          ))}
        </ul>
      )}
      {ok && (
        <p role="status" className="rounded border border-emerald-300 bg-emerald-50 p-2 text-xs text-emerald-800 dark:border-emerald-700 dark:bg-emerald-950 dark:text-emerald-200">
          {ok}
        </p>
      )}
      <button
        type="submit"
        disabled={busy}
        className="rounded bg-neutral-900 px-3 py-2 text-sm text-white dark:bg-neutral-100 dark:text-neutral-900 disabled:opacity-50"
      >
        {busy ? "Registering…" : "Register"}
      </button>
    </form>
  );
}
