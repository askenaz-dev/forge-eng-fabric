"use client";

import { useEffect, useState } from "react";

// Drop-in replacement for free-text tenant_id / workspace_id <input> elements.
// Fetches the signed-in user's accessible scopes from /api/me/{tenants,workspaces}
// and renders a <select>. Falls back to a free-text <input> with a visible
// hint when the scope service is unreachable, so the form still works in
// degraded environments (e.g. control-plane down).
type Tenant = { id: string; name: string };
type Workspace = { id: string; tenant_id: string; name: string };

export type ScopeSelectProps = {
  kind: "tenant" | "workspace";
  name: string;
  defaultValue?: string;
  required?: boolean;
  className?: string;
  style?: React.CSSProperties;
  placeholder?: string;
  // For "workspace" mode: optionally restrict to a tenant. When omitted, all
  // workspaces the user can see are listed.
  tenantId?: string;
  // Controlled mode: when `value` is provided, the component becomes
  // controlled and reports changes via `onChange`. Useful inside drawers/
  // wizards that already track form state.
  value?: string;
  onChange?: (next: string) => void;
};

export function ScopeSelect({
  kind,
  name,
  defaultValue = "",
  required,
  className,
  style,
  placeholder,
  tenantId,
  value: controlledValue,
  onChange,
}: ScopeSelectProps) {
  const controlled = controlledValue !== undefined;
  const [internal, setInternal] = useState(defaultValue);
  const value = controlled ? (controlledValue as string) : internal;
  const setValue = (next: string) => {
    if (!controlled) setInternal(next);
    onChange?.(next);
  };
  const [options, setOptions] = useState<{ value: string; label: string }[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const url =
          kind === "tenant"
            ? "/api/me/tenants"
            : `/api/me/workspaces${tenantId ? `?tenant_id=${encodeURIComponent(tenantId)}` : ""}`;
        const resp = await fetch(url, { cache: "no-store" });
        if (!resp.ok) throw new Error(`${resp.status}`);
        const body = await resp.json();
        if (!alive) return;
        const list = kind === "tenant" ? (body.tenants as Tenant[] | undefined) : (body.workspaces as Workspace[] | undefined);
        const opts = (list ?? []).map((o) => ({ value: o.name, label: o.name }));
        setOptions(opts);
        if (!value && opts.length === 1) setValue(opts[0].value);
      } catch (e) {
        if (alive) {
          setOptions([]);
          setError(e instanceof Error ? e.message : "scope lookup failed");
        }
      }
    })();
    return () => { alive = false; };
  }, [kind, tenantId, value]);

  // While loading: keep the user's typed defaultValue inert in a hidden field
  // so the submit value is stable; render a disabled select for UX feedback.
  if (options === null) {
    return (
      <select disabled className={className} style={style} aria-busy>
        <option>Loading…</option>
      </select>
    );
  }

  if (options.length === 0) {
    return controlled ? (
      <input
        name={name}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        required={required}
        placeholder={placeholder ?? (kind === "tenant" ? "Tenant ID" : "Workspace ID")}
        className={className}
        style={style}
        title={error ? `Scope service unavailable (${error}); type the ID instead.` : undefined}
      />
    ) : (
      <input
        name={name}
        defaultValue={defaultValue}
        required={required}
        placeholder={placeholder ?? (kind === "tenant" ? "Tenant ID" : "Workspace ID")}
        className={className}
        style={style}
        title={error ? `Scope service unavailable (${error}); type the ID instead.` : undefined}
      />
    );
  }

  return (
    <select
      name={name}
      value={value}
      onChange={(e) => setValue(e.target.value)}
      required={required}
      className={className}
      style={style}
    >
      <option value="" disabled>{placeholder ?? (kind === "tenant" ? "Select tenant…" : "Select workspace…")}</option>
      {options.map((o) => (
        <option key={o.value} value={o.value}>{o.label}</option>
      ))}
    </select>
  );
}
