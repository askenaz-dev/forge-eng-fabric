"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { signOut, useSession } from "next-auth/react";
import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { Chev, ChevDown, ForgeMark, More } from "../icons";
import { useLang } from "../providers/LangProvider";
import { NAV_GROUPS, type NavGroupId } from "./nav";
import { cx } from "../primitives/cx";
import { TenantPicker } from "./TenantPicker";

type Counts = {
  agents?: number;
  skills?: number;
  mcp?: number;
  approvals?: number;
};

type PermissionSet = Set<string>;

const COLLAPSE_KEY = "forge_sidebar_groups";

function useGroupCollapse(): {
  isCollapsed: (id: NavGroupId) => boolean;
  toggle: (id: NavGroupId) => void;
} {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  useEffect(() => {
    try {
      const raw = localStorage.getItem(COLLAPSE_KEY);
      if (raw) setCollapsed(JSON.parse(raw));
    } catch {
      // ignore
    }
  }, []);

  function toggle(id: NavGroupId) {
    setCollapsed((prev) => {
      const next = { ...prev, [id]: !prev[id] };
      try {
        localStorage.setItem(COLLAPSE_KEY, JSON.stringify(next));
      } catch {
        // ignore
      }
      return next;
    });
  }

  return {
    isCollapsed: (id: NavGroupId) => !!collapsed[id],
    toggle,
  };
}

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
  const groups = useGroupCollapse();

  const user = session?.user;
  const displayName = user?.name || user?.email || "—";
  const initial = (displayName.match(/[A-Za-zÁ-Úá-ú]/) || ["F"])[0].toUpperCase();
  const subtitle = user?.email && user.email !== displayName ? user.email : t("foot_role");

  // alfred-console-redesign §3.4: developer mode toggle state.
  const [devMode, setDevMode] = useState<boolean | null>(null);
  useEffect(() => {
    fetch("/api/user/preferences", { cache: "no-store" })
      .then((r) => r.ok ? r.json() : null)
      .then((d: { console_view_preference?: string } | null) => {
        setDevMode((d?.console_view_preference ?? "friendly") === "advanced");
      })
      .catch(() => setDevMode(false));
  }, []);

  function toggleDevMode() {
    const next = !devMode;
    setDevMode(next);
    // Persist via the preferences API (§3.5 emits view_toggled event server-side).
    void fetch("/api/user/preferences", {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ console_view_preference: next ? "advanced" : "friendly" }),
    });
    // Switch the active Alfred page if on it.
    if (typeof window !== "undefined" && window.location.pathname.startsWith("/alfred")) {
      window.location.href = `/alfred?view=${next ? "advanced" : "friendly"}`;
    }
  }

  // When the rail is collapsed (icon-only mode) the section labels are hidden,
  // so we always expand the items inside; otherwise respect the per-group state.
  const groupExpanded = useMemo(() => {
    return (id: NavGroupId) => collapsed || !groups.isCollapsed(id);
  }, [collapsed, groups]);

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
          const expanded = groupExpanded(group.id);
          const groupLabel = t(group.labelKey);
          const sectionId = `side-section-${group.id}`;
          return (
            <div key={group.id} className="side-group" data-collapsed={!expanded || undefined}>
              <button
                type="button"
                className="side-section"
                onClick={() => groups.toggle(group.id)}
                aria-expanded={expanded}
                aria-controls={sectionId}
                aria-label={`${groupLabel} — ${expanded ? t("collapse_section") : t("expand_section")}`}
                disabled={collapsed}
              >
                <span>{groupLabel}</span>
                <ChevDown className="caret" aria-hidden />
              </button>
              <div
                id={sectionId}
                className="side-group-items"
                aria-hidden={!expanded}
              >
                <div className="side-group-items-inner">
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
                        tabIndex={expanded ? undefined : -1}
                      >
                        <Icon />
                        <span>{t(item.labelKey)}</span>
                        {count != null && <span className="ct">{count}</span>}
                      </Link>
                    );
                  })}
                </div>
              </div>
            </div>
          );
        })}
      </nav>

      <div className="side-footer">
        <div className="ava" aria-hidden>
          {initial}
        </div>
        <div className="who">
          <b title={displayName}>{displayName}</b>
          <small title={subtitle}>{subtitle}</small>
        </div>
        <DropdownMenu.Root>
          <DropdownMenu.Trigger asChild>
            <button type="button" className="icon-btn" aria-label={t("account_menu")}>
              <More />
            </button>
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content className="pop" align="end" side="right" sideOffset={8} collisionPadding={12}>
              <div className="pop-header">
                <b>{displayName}</b>
                <small>{subtitle}</small>
              </div>
              <DropdownMenu.Separator className="pop-divider" />
              {/* alfred-console-redesign §3.4: developer mode toggle */}
              {devMode !== null && (
                <DropdownMenu.Item
                  className="pop-item"
                  onSelect={(event) => {
                    event.preventDefault();
                    toggleDevMode();
                  }}
                >
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", width: "100%", gap: 12 }}>
                    <span>{t("alfred_dev_mode_toggle")}</span>
                    <span
                      style={{
                        display: "inline-block",
                        width: 32,
                        height: 18,
                        borderRadius: 9,
                        background: devMode ? "var(--primary)" : "var(--bg-hover)",
                        position: "relative",
                        transition: "background 0.15s",
                        flexShrink: 0,
                      }}
                      aria-checked={devMode}
                      role="switch"
                    >
                      <span
                        style={{
                          position: "absolute",
                          top: 2,
                          left: devMode ? 16 : 2,
                          width: 14,
                          height: 14,
                          borderRadius: "50%",
                          background: "white",
                          transition: "left 0.15s",
                          boxShadow: "0 1px 2px rgba(0,0,0,0.2)",
                        }}
                      />
                    </span>
                  </div>
                </DropdownMenu.Item>
              )}
              <DropdownMenu.Separator className="pop-divider" />
              <DropdownMenu.Item
                className="pop-item danger"
                onSelect={(event) => {
                  event.preventDefault();
                  void signOut({ callbackUrl: "/" });
                }}
              >
                <span>{t("sign_out")}</span>
              </DropdownMenu.Item>
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
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
