"use client";

import * as Popover from "@radix-ui/react-popover";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useSession } from "next-auth/react";
import { Check, Globe } from "../icons";
import { useLang } from "../providers/LangProvider";
import { useToast } from "../providers/ToastProvider";
import { cx } from "../primitives/cx";

type Workspace = { id: string; name: string; tenant_id: string };

export function WorkspacePicker({
  activeSlug,
  activeName,
  tenantSlug,
}: {
  activeSlug: string;
  activeName: string;
  tenantSlug: string;
}) {
  const { t } = useLang();
  const toast = useToast();
  const router = useRouter();
  const { update } = useSession();
  const [open, setOpen] = useState(false);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    fetch("/api/command-palette/search?q=", { cache: "no-store" })
      .then((r) => r.json())
      .then((palette) => {
        const workspaceSrc = palette.sources?.find(
          (s: { source: string }) => s.source === "workspaces",
        );
        if (workspaceSrc?.results) {
          setWorkspaces(
            workspaceSrc.results.map(
              (r: { id: string; title: string; subtitle?: string }) => {
                const parts = (r.subtitle ?? "").split(" · ");
                return {
                  id: parts[1] ?? r.id.replace(/^workspace\./, ""),
                  tenant_id: parts[0] ?? "",
                  name: r.title,
                };
              },
            ),
          );
        }
      })
      .catch(() => undefined)
      .finally(() => setLoading(false));
  }, [open]);

  async function pickWorkspace(workspace: Workspace) {
    // Re-sign the next-auth JWT with the new workspace context — server
    // components and API routes will read this via getServerSession().
    await update({ tenantSlug: workspace.tenant_id, workspaceSlug: workspace.id });
    // Audit + control-plane sync (no cookie writes anymore).
    await fetch("/api/workspace/active", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ tenant: workspace.tenant_id, workspace: workspace.id }),
    }).catch(() => undefined);
    toast.success(t("toast_workspace"));
    setOpen(false);
    router.refresh();
  }

  const sameTenant = workspaces.filter((w) => w.tenant_id === tenantSlug);

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button
          className="workspace-pill"
          type="button"
          aria-label={t("workspace_picker_aria")}
        >
          <Globe className="workspace-pill__icon" />
          <span className="workspace-pill__label">{activeName}</span>
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className="pop pop--tenant" align="start" sideOffset={8} collisionPadding={12}>
          <div className="pop-header">
            <small className="pop-label">{t("workspace_active")}</small>
            <b>{activeName}</b>
            <small className="pop-sub">{activeSlug}</small>
          </div>

          <div className="pop-divider" />

          {loading && <div className="pop-state">{t("tenants_loading")}</div>}

          {!loading && sameTenant.length === 0 && (
            <div className="pop-state">{t("tenant_no_workspaces")}</div>
          )}

          {!loading &&
            sameTenant.map((workspace) => {
              const active = workspace.id === activeSlug;
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
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
