// Public TypeScript interface for custom node renderers.
// See docs/sdk/custom-nodes.md.

import type React from "react";

export interface FlowNodeRenderer {
  /** Tailwind tint class for the node accent. */
  tint: string;
  /** Optional icon (SVG component or emoji shorthand). */
  icon?: React.ReactNode;
  /** Header component — usually just the display name + a status indicator. */
  Header: React.ComponentType<{ nodeId: string }>;
  /** Body component — shown inside the node box. */
  Body: React.ComponentType<{ nodeId: string; config: Record<string, unknown> }>;
  /** Property panel — shown in the right rail when the node is selected. */
  PropertyPanel?: React.ComponentType<{
    nodeId: string;
    config: Record<string, unknown>;
    onChange: (next: Record<string, unknown>) => void;
  }>;
}

/**
 * NodeManifest is the parsed shape of a forge-node.yaml. Portal admin
 * registration validates against this before persisting per workspace.
 */
export interface NodeManifest {
  id: string;
  version: string;
  publisher: string;
  display_name: string;
  description?: string;
  category: "trigger" | "ai" | "action" | "logic";
  inputs: JSONSchema;
  outputs: JSONSchema;
  config: JSONSchema;
  permissions: string[];
  endpoint: {
    url: string;
    timeout?: string;
    retries?: { max?: number; backoff?: "constant" | "exponential" };
  };
}

export type JSONSchema = Record<string, unknown>;

/**
 * validateManifest returns null on success or a list of error messages.
 * The validator is intentionally lightweight; the runtime authoritative
 * validation lives in services/workflow-runtime.
 */
export function validateManifest(m: unknown): string[] | null {
  const errs: string[] = [];
  const obj = (m ?? {}) as Record<string, unknown>;
  const required = ["id", "version", "publisher", "display_name", "category", "inputs", "outputs", "config", "permissions", "endpoint"];
  for (const k of required) {
    if (!(k in obj)) errs.push(`missing_required_field: ${k}`);
  }
  if (typeof obj.id === "string" && !/^[a-z][a-z0-9-]*$/.test(obj.id)) {
    errs.push("invalid_id: must be kebab-case");
  }
  if (typeof obj.version === "string" && !/^\d+\.\d+\.\d+/.test(obj.version)) {
    errs.push("invalid_version: must be SemVer");
  }
  if (typeof obj.category === "string" && !["trigger", "ai", "action", "logic"].includes(obj.category)) {
    errs.push(`invalid_category: ${obj.category}`);
  }
  if (errs.length === 0) return null;
  return errs;
}
