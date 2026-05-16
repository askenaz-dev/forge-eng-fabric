"use client";

// CodeViewTab renders the current canonical AST as canonical JSON/YAML
// for direct editing. Edits are pushed back to the canvas via onChange.
// The actual round-trip happens via the ast-canvas-adapter when the
// shell calls canvasToAST/astToCanvas around this surface.

import { useEffect, useState } from "react";

export function CodeViewTab({
  value,
  onChange,
}: {
  value: string;
  onChange: (next: string, parsed: object | null, error: string | null) => void;
}) {
  const [local, setLocal] = useState(value);

  useEffect(() => {
    setLocal(value);
  }, [value]);

  return (
    <div className="absolute inset-0 bg-white dark:bg-neutral-950 p-3 z-10">
      <p className="text-xs opacity-60 mb-2">
        Edits in code view update the canvas on save. Invalid JSON blocks the save.
      </p>
      <textarea
        value={local}
        onChange={(e) => {
          setLocal(e.target.value);
          try {
            const parsed = JSON.parse(e.target.value);
            onChange(e.target.value, parsed, null);
          } catch (err) {
            onChange(e.target.value, null, err instanceof Error ? err.message : String(err));
          }
        }}
        spellCheck={false}
        className="w-full h-[calc(100%-2rem)] font-mono text-xs rounded border border-neutral-300 dark:border-neutral-700 p-3 bg-neutral-50 dark:bg-neutral-900"
      />
    </div>
  );
}
