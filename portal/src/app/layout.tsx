import "./globals.css";
import type { ReactNode } from "react";
import { Providers } from "./providers";

export const metadata = {
  title: "Forge Engineering Fabric",
  description: "Phase 0 portal",
};

const modules = [
  { label: "Workspaces", href: "/" },
  { label: "Alfred Console", href: "/alfred" },
  { label: "Asset Registry", href: "/assets" },
  { label: "OpenSpecs", href: "/openspecs" },
  { label: "Repositories", href: "/settings/github" },
  { label: "Environments", href: "#" },
  { label: "Deployments", href: "#" },
  { label: "Workflows", href: "#" },
  { label: "Approvals Inbox", href: "/approvals" },
  { label: "Observability", href: "#" },
  { label: "Admin & Governance", href: "/permissions" },
];

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-neutral-50 text-neutral-900 dark:bg-neutral-950 dark:text-neutral-100">
        <Providers>
          <header className="flex items-center justify-between border-b border-neutral-200 px-6 py-3 dark:border-neutral-800">
            <h1 className="text-lg font-semibold">Forge Engineering Fabric</h1>
            <a href="/api/auth/signout" className="text-sm underline opacity-70">sign out</a>
          </header>
          <div className="grid min-h-[calc(100vh-53px)] md:grid-cols-[260px_1fr]">
            <aside className="border-b border-neutral-200 bg-white px-4 py-4 dark:border-neutral-800 dark:bg-neutral-900 md:border-b-0 md:border-r">
              <p className="mb-3 text-xs font-semibold uppercase tracking-wide text-neutral-500">Modules</p>
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
        </Providers>
      </body>
    </html>
  );
}
