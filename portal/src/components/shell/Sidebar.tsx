"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { useSession } from "next-auth/react";
import { Chev, ForgeMark, More } from "../icons";
import { useLang } from "../providers/LangProvider";
import { NAV_GROUPS } from "./nav";
import { cx } from "../primitives/cx";
import { TenantPicker } from "./TenantPicker";

type Counts = {
  agents?: number;
  skills?: number;
  mcp?: number;
  approvals?: number;
};

type PermissionSet = Set<string>;

export function Sidebar({
  tenantSlug,
  counts,
  permissions,
  collapsed = false,
  onToggleCollapse,
}: {
  tenantSlug: string;
  counts: Counts;
  permissions: PermissionSet;
  collapsed?: boolean;
  onToggleCollapse?: () => void;
}) {
  const { t } = useLang();
  const pathname = usePathname();
  const { data: session } = useSession();

  const user = session?.user;
  const displayName = user?.name || user?.email || "—";
  const initial = (displayName.match(/[A-Za-zÁ-Úá-ú]/) || ["F"])[0].toUpperCase();

  return (
    <aside className="side" aria-label={t("nav_platform")}>
      <div className="side-brand">
        <ForgeMark width={26} height={26} className="mark" />
        <div className="wd">
          <span className="w">Forge</span>
          <span className="o">Engineering Fabric</span>
        </div>
        <TenantPicker activeSlug={tenantSlug} />
      </div>

      <nav className="side-nav" aria-label="primary">
        {NAV_GROUPS.map((group) => {
          const items = group.items.filter(
            (i) => !i.permission || permissions.has(i.permission),
          );
          if (items.length === 0) return null;
          return (
            <div key={group.id}>
              <div className="side-section">{t(group.labelKey)}</div>
              {items.map((item) => {
                const isActive = isLinkActive(pathname, item.href);
                const Icon = item.icon;
                const count =
                  item.countSource != null ? counts[item.countSource] : undefined;
                return (
                  <Link
                    key={item.id}
                    href={item.href}
                    className={cx("side-link", isActive && "active")}
                    aria-current={isActive ? "page" : undefined}
                  >
                    <Icon />
                    <span>{t(item.labelKey)}</span>
                    {count != null && <span className="ct">{count}</span>}
                  </Link>
                );
              })}
            </div>
          );
        })}
      </nav>

      <div className="side-footer">
        <div className="ava">{initial}</div>
        <div className="who">
          <b>{displayName}</b>
          <small>{t("foot_role")}</small>
        </div>
        <Link href="/api/auth/signout" className="icon-btn" aria-label={t("sign_out")}>
          <More />
        </Link>
        {onToggleCollapse && (
          <button
            type="button"
            className="icon-btn"
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            aria-pressed={collapsed}
            onClick={onToggleCollapse}
            style={{ transform: collapsed ? "rotate(0deg)" : "rotate(180deg)" }}
          >
            <Chev />
          </button>
        )}
      </div>
    </aside>
  );
}

function isLinkActive(pathname: string, href: string): boolean {
  if (!pathname) return false;
  const clean = pathname.split(/[?#]/)[0];
  const target = href.split(/[?#]/)[0];
  if (target === "/") return clean === "/";
  return clean === target || clean.startsWith(`${target}/`);
}

export function useStickyCollapse(): {
  collapsed: boolean;
  toggle: () => void;
} {
  const [collapsed, setCollapsed] = useState(false);

  useEffect(() => {
    const stored = localStorage.getItem("forge_sidebar");
    const initial = stored === "collapsed";
    const tablet = window.matchMedia("(max-width: 1024px)");
    setCollapsed(initial || tablet.matches);
    const handler = (e: MediaQueryListEvent) => setCollapsed(e.matches || stored === "collapsed");
    tablet.addEventListener?.("change", handler);
    return () => tablet.removeEventListener?.("change", handler);
  }, []);

  function toggle() {
    setCollapsed((c) => {
      const next = !c;
      try {
        localStorage.setItem("forge_sidebar", next ? "collapsed" : "expanded");
      } catch {
        // ignored
      }
      return next;
    });
  }

  return { collapsed, toggle };
}
