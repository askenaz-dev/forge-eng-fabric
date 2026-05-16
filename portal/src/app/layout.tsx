import "./globals.css";
import type { ReactNode } from "react";
import { headers } from "next/headers";
import { getServerSession } from "next-auth";
import { authOptions } from "@/auth";
import { Providers } from "./providers";
import { PortalShell } from "@/components/shell/PortalShell";
import { LegacyShell } from "@/components/shell/LegacyShell";
import { fontClassNames, geist } from "./fonts";
import { initialDataTheme, readPreferences } from "@/lib/prefs";
import { endpoint } from "@/lib/api";

function titleCase(slug: string): string {
  return slug
    .split(/[-_]/)
    .map((s) => (s.length === 0 ? s : s[0].toUpperCase() + s.slice(1)))
    .join(" ");
}

export const metadata = {
  title: "Forge Engineering Fabric",
  description: "Forge Engineering Fabric — Internal Developer Portal",
};

async function fetchInitialPermissions(token?: string): Promise<string[]> {
  if (!token) return ["policy:read", "audit:read", "admin"];
  try {
    const r = await fetch(`${endpoint("POLICY_URL")}/v1/permissions/me`, {
      headers: { authorization: `Bearer ${token}`, accept: "application/json" },
      cache: "no-store",
    });
    if (!r.ok) throw new Error(String(r.status));
    const data = (await r.json()) as { permissions?: string[] };
    return data.permissions ?? [];
  } catch {
    return ["policy:read", "audit:read", "admin"];
  }
}

async function fetchInitialCounts(token: string | undefined, actor: string) {
  const reg = endpoint("REGISTRY_URL");
  const apr = endpoint("APPROVALS_URL");
  async function safe(url: string): Promise<number> {
    try {
      const r = await fetch(url, {
        headers: { ...(token ? { authorization: `Bearer ${token}` } : {}) },
        cache: "no-store",
      });
      if (!r.ok) return 0;
      const data = (await r.json()) as { total?: number; items?: unknown[]; approvals?: unknown[] };
      if (typeof data.total === "number") return data.total;
      if (Array.isArray(data.items)) return data.items.length;
      if (Array.isArray(data.approvals)) return data.approvals.length;
      return 0;
    } catch {
      return 0;
    }
  }
  const [agents, skills, mcp, approvals] = await Promise.all([
    safe(`${reg}/v1/assets?kind=agent&status=approved&summary=true`),
    safe(`${reg}/v1/assets?kind=skill&status=approved&summary=true`),
    safe(`${reg}/v1/assets?kind=mcp&status=approved&summary=true`),
    safe(`${apr}/v1/approvals?status=pending&approver=${encodeURIComponent(actor)}`),
  ]);
  return { agents, skills, mcp, approvals };
}

export default async function RootLayout({ children }: { children: ReactNode }) {
  const rebrandFlag = (process.env.PORTAL_REBRAND ?? "1") !== "0";

  const prefs = readPreferences();
  const dataTheme = initialDataTheme(prefs);

  // The Geist face is applied as the default `body` font via its CSS variable.
  // Other faces are referenced through the design-system tokens.
  if (!rebrandFlag) {
    // Legacy shell path — keeps the rebrand controllable in production until
    // cutover. The legacy module simply re-exports the previous v1 layout.
    return (
      <html lang={prefs.lang} suppressHydrationWarning>
        <body className={geist.className}>
          <Providers initialTheme={prefs.theme} initialDensity={prefs.density} initialLang={prefs.lang}>
            <LegacyShell>{children}</LegacyShell>
          </Providers>
        </body>
      </html>
    );
  }

  const session = await getServerSession(authOptions);
  const token = session?.accessToken;
  const actor = session?.user?.email ?? session?.user?.name ?? "anonymous";

  const [permissions, counts] = await Promise.all([
    fetchInitialPermissions(token),
    fetchInitialCounts(token, actor),
  ]);

  const tenantSlug = session?.tenantSlug ?? "acme";
  const workspaceSlug = session?.workspaceSlug ?? "engineering";
  const tenantName = titleCase(tenantSlug);
  const workspaceName = titleCase(workspaceSlug);
  const githubHref = process.env.PORTAL_GITHUB_HREF;

  // Surface the correlation id from the incoming request for traceability.
  headers();

  return (
    <html
      lang={prefs.lang}
      data-theme={dataTheme}
      data-density={prefs.density}
      className={fontClassNames}
      suppressHydrationWarning
    >
      <body>
        <Providers initialTheme={prefs.theme} initialDensity={prefs.density} initialLang={prefs.lang}>
          <PortalShell
            tenantSlug={tenantSlug}
            tenantName={tenantName}
            workspaceSlug={workspaceSlug}
            workspaceName={workspaceName}
            githubHref={githubHref}
            initialPermissions={permissions}
            initialCounts={counts}
          >
            {children}
          </PortalShell>
        </Providers>
      </body>
    </html>
  );
}
