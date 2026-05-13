import type { DictKey } from "@/i18n/dictionary";

export type PaletteSourceId =
  | "nav"
  | "agents"
  | "skills"
  | "mcp"
  | "runs"
  | "approvals"
  | "specs"
  | "workspaces"
  | "tenants"
  | "actions";

export type PaletteResult = {
  id: string;
  source: PaletteSourceId;
  title: string;
  subtitle?: string;
  hrefOrAction:
    | { kind: "navigate"; href: string }
    | { kind: "action"; action: PaletteAction };
  score?: number;
};

export type PaletteAction =
  | { type: "theme"; theme: "light" | "dark" | "system" }
  | { type: "density"; density: "compact" | "comfortable" | "spacious" }
  | { type: "lang"; lang: "es" | "en" }
  | { type: "sidebar" }
  | { type: "sign-out" }
  | { type: "workspace"; tenant: string; workspace: string };

export type PaletteSourceResponse = {
  source: PaletteSourceId;
  status: "ok" | "unreachable";
  results: PaletteResult[];
};

export type PaletteGroupLabel = {
  source: PaletteSourceId;
  labelKey: DictKey;
};

export const PALETTE_GROUP_LABELS: PaletteGroupLabel[] = [
  { source: "nav",        labelKey: "cmd_group_nav" },
  { source: "actions",    labelKey: "cmd_group_actions" },
  { source: "agents",     labelKey: "cmd_group_agents" },
  { source: "skills",     labelKey: "cmd_group_skills" },
  { source: "mcp",        labelKey: "cmd_group_mcp" },
  { source: "runs",       labelKey: "cmd_group_runs" },
  { source: "approvals",  labelKey: "cmd_group_approvals" },
  { source: "specs",      labelKey: "cmd_group_specs" },
  { source: "workspaces", labelKey: "cmd_group_workspaces" },
  { source: "tenants",    labelKey: "cmd_group_tenants" },
];
