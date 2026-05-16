"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useLang } from "@/components/providers/LangProvider";
import { Button } from "@/components/primitives";
import { Plus, User } from "@/components/icons";

export function DashboardHeadline({
  agentsActive,
  approvalsPending,
}: {
  agentsActive: number;
  approvalsPending: number;
}) {
  const { t } = useLang();
  const router = useRouter();
  const [navigating, setNavigating] = useState<string | null>(null);

  function go(href: string) {
    setNavigating(href);
    router.push(href);
  }

  return (
    <div className="page-head">
      <div>
        <h1 className="page-title">
          {t("h_overview_pre")} <em>{t("h_overview_em")}</em>{" "}
          {t("h_overview_post", { agents: agentsActive, approvals: approvalsPending })
            .replace(/^([^A-Za-z]*[A-Za-z])/, "$1")}
        </h1>
      </div>
      <div className="page-meta">
        <Button
          variant="secondary"
          leading={navigating === "/workspaces/new" ? <span className="spinner" /> : <User />}
          disabled={navigating !== null}
          onClick={() => go("/workspaces/new")}
        >
          {t("h_invite")}
        </Button>
        <Button
          variant="primary"
          leading={navigating === "/workflows" ? <span className="spinner" /> : <Plus />}
          disabled={navigating !== null}
          onClick={() => go("/workflows")}
        >
          {t("h_new_run")}
        </Button>
      </div>
    </div>
  );
}
