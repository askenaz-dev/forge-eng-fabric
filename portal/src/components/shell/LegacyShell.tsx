"use client";

// Legacy V1 shell — preserved behind the PORTAL_REBRAND=0 flag during the
// cutover window described in design.md. Removal is the responsibility of
// the follow-up archive change (tasks 15.1–15.4).

import type { ReactNode } from "react";

const modules = [
  { label: "Workspaces", href: "/" },
  { label: "New App", href: "/apps/new" },
  { label: "Onboarding History", href: "/onboarding" },
  { label: "Templates", href: "/templates" },
  { label: "PR Gates", href: "/pr-gates" },
  { label: "Alfred Console", href: "/alfred" },
  { label: "Alfred Wizard (Beta)", href: "/alfred/wizard?wizard=1" },
  { label: "Asset Registry", href: "/assets" },
  { label: "Specifications", href: "/openspecs" },
  { label: "Initiatives", href: "/initiatives" },
  { label: "Repositories", href: "/settings/github" },
  { label: "Runtimes", href: "/runtimes" },
  { label: "Deployments", href: "/deployments" },
  { label: "Drift", href: "/drift" },
  { label: "Environments", href: "#" },
  { label: "Workflows", href: "/workflows" },
  { label: "Workflow Editor (Beta)", href: "/workflows/editor" },
  { label: "Marketplace", href: "/marketplace" },
  { label: "Approvals Inbox", href: "/approvals" },
  { label: "Observability", href: "#" },
  { label: "Incidents", href: "/incidents" },
  { label: "Evolution Inbox", href: "/evolution" },
  { label: "FinOps Recommendations", href: "/finops-recommendations" },
  { label: "Kill Switch", href: "/kill-switch" },
  { label: "Admin & Governance", href: "/permissions" },
];

export function LegacyShell({ children }: { children: ReactNode }) {
  return (
    <>
      <header className="flex items-center justify-between border-b border-neutral-200 px-6 py-3 dark:border-neutral-800">
        <h1 className="text-lg font-semibold">Forge Engineering Fabric</h1>
        <a href="/api/auth/signout" className="text-sm underline opacity-70">
          sign out
        </a>
      </header>
      <div className="grid min-h-[calc(100vh-53px)] md:grid-cols-[260px_1fr]">
        <aside className="border-b border-neutral-200 bg-white px-4 py-4 dark:border-neutral-800 dark:bg-neutral-900 md:border-b-0 md:border-r">
          <p className="mb-3 text-xs font-semibold uppercase tracking-wide text-neutral-500">
            Modules
          </p>
          <nav className="grid gap-1">
            {modules.map((module) => (
              <a
                key={module.label}
                href={module.href}
                className="rounded px-3 py-2 text-sm text-neutral-700 hover:bg-neutral-100 dark:text-neutral-200 dark:hover:bg-neutral-800"
              >
                {module.label}
              </a>
            ))}
          </nav>
        </aside>
        <main className="px-6 py-6">{children}</main>
      </div>
    </>
  );
}
