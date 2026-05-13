"use client";

import { useLang } from "@/components/providers/LangProvider";
import { Button } from "@/components/primitives";
import { Plus, User } from "@/components/icons";
import Link from "next/link";

export function DashboardHeadline({
  agentsActive,
  approvalsPending,
}: {
  agentsActive: number;
  approvalsPending: number;
}) {
  const { t } = useLang();
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
        <Link href="/workspaces/new">
          <Button variant="secondary" leading={<User />}>
            {t("h_invite")}
          </Button>
        </Link>
        <Link href="/workflows">
          <Button variant="primary" leading={<Plus />}>
            {t("h_new_run")}
          </Button>
        </Link>
      </div>
    </div>
  );
}
