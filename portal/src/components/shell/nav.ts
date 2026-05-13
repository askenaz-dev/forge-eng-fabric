import type { ComponentType, SVGProps } from "react";
import {
  Agents,
  Approvals,
  Audit,
  Bolt,
  Branch,
  Globe,
  Home,
  Mcp,
  Obs,
  Plus,
  Policy,
  Settings,
  Skills,
  Specs,
  Terminal,
  Workflows,
  User,
} from "../icons";
import type { DictKey } from "@/i18n/dictionary";

export type NavGroupId = "platform" | "govern" | "observe" | "account";

export type NavItem = {
  id: string;
  href: string;
  labelKey: DictKey;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
  permission?: string;
  countSource?: "agents" | "skills" | "mcp" | "approvals";
};

export type NavGroup = {
  id: NavGroupId;
  labelKey: DictKey;
  items: NavItem[];
};

export const NAV_GROUPS: NavGroup[] = [
  {
    id: "platform",
    labelKey: "nav_platform",
    items: [
      { id: "dashboard",   href: "/",                  labelKey: "nav_dashboard",   icon: Home },
      { id: "workspaces",  href: "/workspaces/new",    labelKey: "nav_workspaces",  icon: Globe },
      { id: "alfred",      href: "/alfred",            labelKey: "nav_alfred",      icon: Terminal },
      { id: "agents",      href: "/assets?kind=agent", labelKey: "nav_agents",      icon: Agents, countSource: "agents" },
      { id: "skills",      href: "/assets?kind=skill", labelKey: "nav_skills",      icon: Skills, countSource: "skills" },
      { id: "mcp",         href: "/assets?kind=mcp",   labelKey: "nav_mcp",         icon: Mcp,    countSource: "mcp" },
      { id: "gateway",     href: "/gateway",           labelKey: "nav_gateway",     icon: Globe },
      { id: "workflows",   href: "/workflows",         labelKey: "nav_workflows",   icon: Workflows },
      { id: "marketplace", href: "/marketplace",       labelKey: "nav_marketplace", icon: Plus },
      { id: "templates",   href: "/templates",         labelKey: "nav_templates",   icon: Specs },
      { id: "apps-new",    href: "/apps/new",          labelKey: "nav_apps_new",    icon: Plus },
      { id: "onboarding",  href: "/onboarding",        labelKey: "nav_onboarding",  icon: Branch },
    ],
  },
  {
    id: "govern",
    labelKey: "nav_govern",
    items: [
      { id: "approvals",   href: "/approvals",         labelKey: "nav_approvals",   icon: Approvals, countSource: "approvals" },
      { id: "specs",       href: "/openspecs",         labelKey: "nav_specs",       icon: Specs },
      { id: "initiatives", href: "/initiatives",       labelKey: "nav_initiatives", icon: Specs },
      { id: "policies",    href: "/permissions",       labelKey: "nav_policies",    icon: Policy, permission: "policy:read" },
      { id: "audit",       href: "/permissions?tab=audit", labelKey: "nav_audit",   icon: Audit,  permission: "audit:read" },
      { id: "pr-gates",    href: "/pr-gates",          labelKey: "nav_pr_gates",    icon: Approvals },
      { id: "kill-switch", href: "/kill-switch",       labelKey: "nav_kill_switch", icon: Bolt,   permission: "admin" },
    ],
  },
  {
    id: "observe",
    labelKey: "nav_observe",
    items: [
      { id: "obs",         href: "/incidents",                labelKey: "nav_obs",         icon: Obs },
      { id: "incidents",   href: "/incidents",                labelKey: "nav_incidents",   icon: Audit },
      { id: "deployments", href: "/deployments",              labelKey: "nav_deployments", icon: Branch },
      { id: "drift",       href: "/drift",                    labelKey: "nav_drift",       icon: Workflows },
      { id: "evolution",   href: "/evolution",                labelKey: "nav_evolution",   icon: Specs },
      { id: "finops",      href: "/finops-recommendations",   labelKey: "nav_finops",      icon: Bolt },
      { id: "runtimes",    href: "/runtimes",                 labelKey: "nav_runtimes",    icon: Globe },
    ],
  },
  {
    id: "account",
    labelKey: "nav_account",
    items: [
      { id: "settings",    href: "/settings/github",          labelKey: "nav_settings",    icon: Settings },
      { id: "permissions", href: "/permissions",              labelKey: "nav_permissions", icon: User, permission: "admin" },
      { id: "tenants",     href: "/admin/tenants",            labelKey: "nav_tenants",     icon: Globe, permission: "admin" },
      { id: "assets",      href: "/assets",                   labelKey: "nav_assets",      icon: Specs },
    ],
  },
];

export function findNavItem(pathname: string): NavItem | undefined {
  // Strip query / hash and normalise trailing slashes.
  const clean = pathname.split(/[?#]/)[0].replace(/\/$/, "") || "/";
  for (const group of NAV_GROUPS) {
    for (const item of group.items) {
      const itemHref = item.href.split(/[?#]/)[0].replace(/\/$/, "") || "/";
      if (clean === itemHref) return item;
    }
  }
  // Fall back to the deepest matching prefix.
  let best: NavItem | undefined;
  for (const group of NAV_GROUPS) {
    for (const item of group.items) {
      const itemHref = item.href.split(/[?#]/)[0].replace(/\/$/, "") || "/";
      if (clean.startsWith(itemHref) && (!best || itemHref.length > best.href.length)) {
        best = item;
      }
    }
  }
  return best;
}
