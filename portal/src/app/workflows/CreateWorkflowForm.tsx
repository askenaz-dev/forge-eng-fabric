"use client";

import { useEffect, useMemo, useState, useTransition } from "react";

// Client form so the Create button only activates once every required value
// is present, regardless of whether tenant/workspace were seeded by the URL
// or typed inline. Submitting kebab-cases the workflow id and shows a
// spinner while the server action is in flight.
export type CreateWorkflowFormProps = {
  tenantId: string;
  workspaceId: string;
  action: (formData: FormData) => Promise<void> | void;
};

type TenantOption = { id: string; name: string };
type WorkspaceOption = { id: string; tenant_id: string; name: string };

const ID_RE = /^[a-z0-9][a-z0-9-]*$/;

export function CreateWorkflowForm({ tenantId, workspaceId, action }: CreateWorkflowFormProps) {
  const [tenant, setTenant] = useState(tenantId);
  const [workspace, setWorkspace] = useState(workspaceId);
  const [tenants, setTenants] = useState<TenantOption[]>([]);
  const [workspaces, setWorkspaces] = useState<WorkspaceOption[]>([]);
  const [scopeError, setScopeError] = useState<string | null>(null);
  const [id, setId] = useState("");
  const [name, setName] = useState("");
  const [visibility, setVisibility] = useState("workspace");
  const [description, setDescription] = useState("");
  const [pending, startTransition] = useTransition();

  // Load the tenants the signed-in user can see. The /api/me proxies forward
  // the session token to control-plane; on failure we fall back to letting
  // the user type the IDs in (preserves the original UX as an escape hatch).
  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const resp = await fetch("/api/me/tenants", { cache: "no-store" });
        if (!resp.ok) throw new Error(`${resp.status}`);
        const body = (await resp.json()) as { tenants?: TenantOption[] };
        if (!alive) return;
        const list = body.tenants ?? [];
        setTenants(list);
        if (!tenant && list.length === 1) setTenant(list[0].name);
      } catch (e) {
        if (alive) setScopeError(e instanceof Error ? e.message : "tenant lookup failed");
      }
    })();
    return () => { alive = false; };
  }, [tenant]);

  // Reload workspaces whenever the selected tenant changes. We filter by
  // tenant on the proxy side too, but matching here keeps the dropdown
  // responsive when the user switches tenant.
  useEffect(() => {
    if (!tenant) { setWorkspaces([]); return; }
    let alive = true;
    const selected = tenants.find((t) => t.name === tenant || t.id === tenant);
    const qs = selected ? `?tenant_id=${encodeURIComponent(selected.id)}` : "";
    (async () => {
      try {
        const resp = await fetch(`/api/me/workspaces${qs}`, { cache: "no-store" });
        if (!resp.ok) throw new Error(`${resp.status}`);
        const body = (await resp.json()) as { workspaces?: WorkspaceOption[] };
        if (!alive) return;
        const list = body.workspaces ?? [];
        setWorkspaces(list);
        if (!workspace && list.length === 1) setWorkspace(list[0].name);
      } catch (e) {
        if (alive) setScopeError(e instanceof Error ? e.message : "workspace lookup failed");
      }
    })();
    return () => { alive = false; };
  }, [tenant, tenants, workspace]);

  const idValid = useMemo(() => (id ? ID_RE.test(id) : true), [id]);

  const missing = useMemo(() => {
    const m: string[] = [];
    if (tenant.trim().length === 0) m.push("Tenant ID");
    if (workspace.trim().length === 0) m.push("Workspace ID");
    if (id.trim().length === 0) m.push("Workflow ID");
    if (!idValid) m.push("valid Workflow ID");
    if (name.trim().length === 0) m.push("Display name");
    return m;
  }, [tenant, workspace, id, idValid, name]);

  const canSubmit = !pending && missing.length === 0;

  return (
    <form
      action={(formData) => {
        if (!canSubmit) return;
        startTransition(() => {
          void action(formData);
        });
      }}
      className="form-card"
      aria-busy={pending}
    >
      <div className="form-card-title">
        <span>New workflow</span>
        {pending && <span className="spinner" aria-label="Creating" />}
      </div>

      {(!tenantId || !workspaceId) && (
        <>
          <label className="fld">
            <span className="fld-label">Tenant</span>
            {tenants.length > 0 ? (
              <select
                name="tenant_id"
                value={tenant}
                onChange={(event) => { setTenant(event.target.value); setWorkspace(""); }}
                className="fld-select"
                required
              >
                <option value="" disabled>Select tenant…</option>
                {tenants.map((t) => (
                  <option key={t.id} value={t.name}>{t.name}</option>
                ))}
              </select>
            ) : (
              <input
                name="tenant_id"
                value={tenant}
                onChange={(event) => setTenant(event.target.value)}
                placeholder="acme"
                className="fld-input fld-input--mono"
                required
              />
            )}
            {scopeError && tenants.length === 0 && (
              <span className="fld-hint">Could not load tenant list ({scopeError}); type the ID instead.</span>
            )}
          </label>
          <label className="fld">
            <span className="fld-label">Workspace</span>
            {workspaces.length > 0 ? (
              <select
                name="workspace_id"
                value={workspace}
                onChange={(event) => setWorkspace(event.target.value)}
                className="fld-select"
                required
              >
                <option value="" disabled>Select workspace…</option>
                {workspaces.map((w) => (
                  <option key={w.id} value={w.name}>{w.name}</option>
                ))}
              </select>
            ) : (
              <input
                name="workspace_id"
                value={workspace}
                onChange={(event) => setWorkspace(event.target.value)}
                placeholder="engineering"
                className="fld-input fld-input--mono"
                required
              />
            )}
          </label>
        </>
      )}
      {tenantId && <input type="hidden" name="tenant_id" value={tenantId} />}
      {workspaceId && <input type="hidden" name="workspace_id" value={workspaceId} />}

      <label className="fld">
        <span className="fld-label">Workflow ID</span>
        <input
          name="id"
          value={id}
          onChange={(event) => setId(event.target.value.toLowerCase().replace(/[^a-z0-9-]+/g, "-"))}
          placeholder="release-train"
          className="fld-input fld-input--mono"
          required
        />
        {id && !idValid ? (
          <span className="fld-error">Must start with a-z 0-9 and use only lowercase letters, numbers, and dashes.</span>
        ) : (
          <span className="fld-hint">kebab-case — used in URLs and exports.</span>
        )}
      </label>

      <label className="fld">
        <span className="fld-label">Display name</span>
        <input
          name="name"
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="Release train"
          className="fld-input"
          required
        />
      </label>

      <label className="fld">
        <span className="fld-label">Visibility</span>
        <select
          name="visibility"
          value={visibility}
          onChange={(event) => setVisibility(event.target.value)}
          className="fld-select"
        >
          <option value="private">private</option>
          <option value="workspace">workspace</option>
          <option value="tenant">tenant</option>
        </select>
      </label>

      <label className="fld">
        <span className="fld-label">Description</span>
        <textarea
          name="description"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          rows={2}
          placeholder="What this workflow orchestrates"
          className="fld-textarea"
        />
      </label>

      <button
        type="submit"
        disabled={!canSubmit}
        className="btn btn--primary"
        style={{ justifyContent: "center", width: "100%", opacity: canSubmit ? 1 : 0.45 }}
      >
        {pending ? (
          <>
            <span className="spinner" aria-hidden />
            <span>Creating…</span>
          </>
        ) : (
          "Create workflow"
        )}
      </button>
      {!canSubmit && !pending && missing.length > 0 && (
        <span className="fld-hint" role="status" aria-live="polite">
          Missing: {missing.join(", ")}
        </span>
      )}
    </form>
  );
}
