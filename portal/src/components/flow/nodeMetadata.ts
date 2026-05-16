// Presentation metadata for every canonical step type. The canvas's
// generic FlowNode component looks up label + tint + family by type.
// Keep this in sync with pkg/workflow/ast/catalog.json — the
// CANONICAL_STEP_TYPES check in the parity test catches additions but
// not metadata drift; this map is the single source for display.

import type { CanonicalStepType, CanonicalTriggerType } from "@/lib/ast-canvas-adapter";

export type NodeFamily = "trigger" | "ai" | "action" | "logic" | "custom";

export interface NodeMeta {
  label: string;
  family: NodeFamily;
  tint: string; // tailwind-like classname for the node accent
}

export const STEP_NODE_META: Record<CanonicalStepType, NodeMeta> = {
  // AI family
  llm:                  { label: "LLM",                  family: "ai",     tint: "bg-purple-100 border-purple-400" },
  agent:                { label: "Agent",                family: "ai",     tint: "bg-amber-100 border-amber-400" },
  "prompt-template":    { label: "Prompt Template",      family: "ai",     tint: "bg-indigo-100 border-indigo-400" },

  // Action family
  mcp:                  { label: "MCP",                  family: "action", tint: "bg-blue-100 border-blue-400" },
  skill:                { label: "Skill",                family: "action", tint: "bg-emerald-100 border-emerald-400" },
  webhook:              { label: "Webhook",              family: "action", tint: "bg-teal-100 border-teal-400" },
  "github-action":      { label: "GitHub Action",        family: "action", tint: "bg-neutral-100 border-neutral-400" },
  "deploy-action":      { label: "Deploy",               family: "action", tint: "bg-lime-100 border-lime-400" },
  "approval-action":    { label: "Approval",             family: "action", tint: "bg-rose-100 border-rose-400" },
  "notification-action":{ label: "Notification",         family: "action", tint: "bg-pink-100 border-pink-400" },

  // Logic family
  branch:               { label: "Branch",               family: "logic",  tint: "bg-sky-100 border-sky-400" },
  loop:                 { label: "Loop",                 family: "logic",  tint: "bg-sky-100 border-sky-400" },
  "human-in-the-loop":  { label: "HITL Gate",            family: "logic",  tint: "bg-rose-100 border-rose-400" },
  eval:                 { label: "Eval",                 family: "logic",  tint: "bg-violet-100 border-violet-400" },
  "sub-workflow":       { label: "Sub-flow",             family: "logic",  tint: "bg-neutral-100 border-neutral-400" },

  // Custom (extensibility)
  custom:               { label: "Custom",               family: "custom", tint: "bg-gray-100 border-gray-400" },
};

export const TRIGGER_NODE_META: Record<CanonicalTriggerType, NodeMeta> = {
  manual:        { label: "Manual run",   family: "trigger", tint: "bg-orange-100 border-orange-400" },
  cron:          { label: "Schedule",     family: "trigger", tint: "bg-orange-100 border-orange-400" },
  "webhook-in":  { label: "Webhook in",   family: "trigger", tint: "bg-orange-100 border-orange-400" },
  "event-bus":   { label: "Event bus",    family: "trigger", tint: "bg-orange-100 border-orange-400" },
  "email-inbound": { label: "Email",      family: "trigger", tint: "bg-orange-100 border-orange-400" },
};

export function familyOrder(): NodeFamily[] {
  return ["trigger", "ai", "action", "logic", "custom"];
}

export function familyTitle(f: NodeFamily): string {
  switch (f) {
    case "trigger": return "Triggers";
    case "ai":      return "AI";
    case "action":  return "Actions";
    case "logic":   return "Logic";
    case "custom":  return "Custom";
  }
}
