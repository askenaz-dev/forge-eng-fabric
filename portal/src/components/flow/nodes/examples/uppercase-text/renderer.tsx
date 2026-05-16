"use client";

// Example custom node renderer shipped with the custom-node SDK.
// See docs/sdk/custom-nodes.md.
//
// This file demonstrates the renderer contract but is intentionally
// kept tiny — the real implementation lives in the publisher's repo.
// The Portal canvas registers it under the palette's "Custom" section
// when ENABLE_EXAMPLE_CUSTOM_NODES is set.

import type { FlowNodeRenderer } from "../../../customNodeApi";

export const UppercaseTextRenderer: FlowNodeRenderer = {
  tint: "bg-amber-100 border-amber-400",
  Header: ({ nodeId }) => (
    <div className="px-3 py-1">
      <p className="text-[10px] uppercase tracking-wider opacity-60">Custom · Example</p>
      <p className="text-sm font-medium">Uppercase Text</p>
      <p className="font-mono text-[10px] opacity-60">{nodeId}</p>
    </div>
  ),
  Body: ({ config }) => (
    <div className="px-3 pb-2 text-[10px] opacity-70">
      locale: <code>{String(config.locale ?? "en")}</code>
    </div>
  ),
  PropertyPanel: ({ config, onChange }) => (
    <label className="block text-xs">
      <span className="block text-[10px] uppercase tracking-wider opacity-60 mb-1">Locale</span>
      <input
        type="text"
        value={String(config.locale ?? "en")}
        onChange={(e) => onChange({ ...config, locale: e.target.value })}
        className="w-full rounded border border-neutral-300 dark:border-neutral-700 px-2 py-1 bg-transparent"
      />
    </label>
  ),
};

export default UppercaseTextRenderer;
