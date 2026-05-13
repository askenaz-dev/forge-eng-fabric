"use client";

import * as Popover from "@radix-ui/react-popover";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Check, Diamond, Globe } from "../icons";
import { useLang } from "../providers/LangProvider";
import { useToast } from "../providers/ToastProvider";
import { cx } from "../primitives/cx";

type Tenant = { id: string; name: string };
type Workspace = { id: string; name: string; tenant_id: string };

export function TenantPicker({ activeSlug }: { activeSlug: string }) {
  const { t } = useLang();
  const toast = useToast();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [activeTenant, setActiveTenant] = useState<string>(activeSlug);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    Promise.all([
      fetch("/api/command-palette/search?q=", { cache: "no-store" }).then((r) => r.json()),
    ])
      .then(([palette]) => {
        const tenantSrc = palette.sources?.find((s: { source: string }) => s.source === "tenants");
        const workspaceSrc = palette.sources?.find((s: { source: string }) => s.source === "workspaces");
        if (tenantSrc?.results) {
          setTenants(
            tenantSrc.results.map((r: { id: string; title: string; subtitle?: string }) => ({
              id: r.subtitle ?? r.id.replace(/^tenant\./, ""),
              name: r.title,
            })),
          );
        }
        if (workspaceSrc?.results) {
          setWorkspaces(
            workspaceSrc.results.map((r: { id: string; title: string; subtitle?: string }) => {
              const parts = (r.subtitle ?? "").split(" · ");
              return {
                id: parts[1] ?? r.id.replace(/^workspace\./, ""),
                tenant_id: parts[0] ?? "",
                name: r.title,
              };
            }),
          );
        }
      })
      .catch(() => undefined)
      .finally(() => setLoading(false));
  }, [open]);

  async function pickWorkspace(workspace: Workspace) {
    await fetch("/api/workspace/active", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ tenant: workspace.tenant_id, workspace: workspace.id }),
    }).catch(() => undefined);
    setActiveTenant(workspace.tenant_id);
    toast.success(t("toast_workspace"));
    setOpen(false);
    router.refresh();
  }

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button className="tenant" type="button" title={t("nav_workspaces")}>
          <Diamond /> {activeTenant}
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className="pop" align="start" sideOffset={6} style={{ minWidth: 260 }}>
          {loading && <div className="cmdk-empty">…</div>}
          {!loading && tenants.length === 0 && (
            <div className="cmdk-empty">{t("unavailable")}</div>
          )}
          {tenants.map((tenant) => {
            const tenantWorkspaces = workspaces.filter((w) => w.tenant_id === tenant.id);
            return (
              <div key={tenant.id} style={{ padding: "4px 0" }}>
                <div className="cmdk-group-heading">
                  <Diamond style={{ display: "inline-block", width: 11, height: 11, marginRight: 6, verticalAlign: -1 }} />
                  {tenant.name}
                </div>
                {tenantWorkspaces.length === 0 && (
                  <div className="pop-item" style={{ opacity: 0.6 }}>
                    <small style={{ marginLeft: 0 }}>{t("apr_no_items")}</small>
                  </div>
                )}
                {tenantWorkspaces.map((workspace) => {
                  const active = workspace.tenant_id === activeTenant;
                  return (
                    <button
                      key={`${workspace.tenant_id}.${workspace.id}`}
                      type="button"
                      className={cx("pop-item", active && "active")}
                      onClick={() => pickWorkspace(workspace)}
                    >
                      <Globe className="lead" />
                      <span>{workspace.name}</span>
                      {active && (
                        <span className="check">
                          <Check style={{ width: 13, height: 13 }} />
                        </span>
                      )}
                    </button>
                  );
                })}
              </div>
            );
          })}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
