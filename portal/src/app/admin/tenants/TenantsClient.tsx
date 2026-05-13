"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Button, Card } from "@/components/primitives";
import { useToast } from "@/components/providers/ToastProvider";

type Tenant = { id: string; name: string; created_at?: string };

export function TenantsClient({ initialTenants }: { initialTenants: Tenant[] }) {
  const router = useRouter();
  const toast = useToast();
  const [tenants, setTenants] = useState<Tenant[]>(initialTenants);
  const [name, setName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function create() {
    const trimmed = name.trim();
    if (!trimmed) return;
    setSubmitting(true);
    setError(null);
    try {
      const resp = await fetch("/api/admin/tenants", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ name: trimmed }),
      });
      const payload = (await resp.json().catch(() => ({}))) as Tenant & { message?: string; code?: string };
      if (!resp.ok) {
        throw new Error(payload.message || payload.code || `control-plane ${resp.status}`);
      }
      setTenants((current) => [payload, ...current.filter((t) => t.id !== payload.id)]);
      setName("");
      toast.success(`Tenant "${payload.name}" created`);
      router.refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "create failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="grid gap-5 lg:grid-cols-[1fr_320px]">
      <Card>
        <div className="p-4">
          <h2 className="text-lg font-semibold">Tenants ({tenants.length})</h2>
          <p className="text-sm opacity-70">Each tenant is an isolation boundary; workspaces and assets live inside one.</p>
        </div>
        <div className="border-t border-neutral-200 dark:border-neutral-800">
          {tenants.length === 0 && (
            <p className="p-4 text-sm opacity-60">No tenants yet. Create the first one on the right.</p>
          )}
          {tenants.map((tenant) => (
            <div key={tenant.id} className="flex items-center justify-between gap-4 border-b border-neutral-200 px-4 py-3 last:border-0 dark:border-neutral-800">
              <div className="min-w-0">
                <p className="font-medium">{tenant.name}</p>
                <p className="font-mono text-xs opacity-60">{tenant.id}</p>
              </div>
              {tenant.created_at && (
                <span className="font-mono text-xs opacity-60">{tenant.created_at.slice(0, 10)}</span>
              )}
            </div>
          ))}
        </div>
      </Card>

      <Card>
        <div className="p-4">
          <h2 className="text-lg font-semibold">New tenant</h2>
          <p className="text-sm opacity-70">Requires the platform-admin role on the control-plane.</p>
        </div>
        <div className="border-t border-neutral-200 p-4 dark:border-neutral-800">
          {error && (
            <p className="mb-3 rounded border border-red-300 bg-red-50 p-2 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
              {error}
            </p>
          )}
          <label className="block">
            <span className="mb-1 block text-xs uppercase tracking-wide opacity-70">Name</span>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="acme-engineering"
              className="w-full rounded border border-neutral-300 bg-transparent px-2 py-1.5 text-sm outline-none focus:border-neutral-500 dark:border-neutral-700 dark:focus:border-neutral-400"
              autoFocus
            />
          </label>
          <div className="mt-3 flex justify-end">
            <Button variant="primary" onClick={create} disabled={submitting || !name.trim()}>
              {submitting ? "Creating…" : "Create"}
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}
