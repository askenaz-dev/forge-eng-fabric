"use client";

import * as Popover from "@radix-ui/react-popover";
import { useEffect, useState } from "react";
import Link from "next/link";
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
    fetch("/api/command-palette/search?q=", { cache: "no-store" })
      .then((r) => r.json())
      .then((palette) => {
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

  // Order: active tenant first, then the rest alphabetically.
  const orderedTenants = [...tenants].sort((a, b) => {
    if (a.id === activeTenant) return -1;
    if (b.id === activeTenant) return 1;
    return a.name.localeCompare(b.name);
  });
  const activeTenantInfo = tenants.find((tn) => tn.id === activeTenant);

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button className="tenant" type="button" title={t("nav_workspaces")}>
          <Diamond /> {activeTenant}
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className="pop pop--tenant" align="start" sideOffset={8} collisionPadding={12}>
          <div className="pop-header">
            <small className="pop-label">{t("tenant_active")}</small>
            <b>{activeTenantInfo?.name ?? activeTenant}</b>
            <small className="pop-sub">{activeTenant}</small>
          </div>

          <div className="pop-divider" />

          {loading && <div className="pop-state">{t("tenants_loading")}</div>}

          {!loading && orderedTenants.length === 0 && (
            <div className="pop-state">{t("tenant_no_others")}</div>
          )}

          {!loading && orderedTenants.map((tenant) => {
            const tenantWorkspaces = workspaces.filter((w) => w.tenant_id === tenant.id);
            const isActiveTenant = tenant.id === activeTenant;
            return (
              <div key={tenant.id} className="pop-section">
                <div className="cmdk-group-heading pop-tenant-heading">
                  <Diamond style={{ width: 11, height: 11 }} />
                  <span>{tenant.name}</span>
                  {isActiveTenant && <span className="pop-active-flag">ACTIVE</span>}
                </div>
                {tenantWorkspaces.length === 0 ? (
                  <div className="pop-state pop-state--sub">{t("tenant_no_workspaces")}</div>
                ) : (
                  tenantWorkspaces.map((workspace) => {
                    const active = isActiveTenant;
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
                  })
                )}
              </div>
            );
          })}

          <div className="pop-divider" />
          <Link href="/admin/tenants" className="pop-item pop-item--footer" onClick={() => setOpen(false)}>
            <Diamond className="lead" />
            <span>{t("tenant_manage")}</span>
          </Link>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
