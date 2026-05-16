"use client";

/**
 * DesignSystemPickerForm — client wrapper that adapts the shared
 * `DesignSystemPicker` component to a Next.js server-action form.
 *
 * alfred-design-system-picker (6.4): the /alfred/wizard route is a Server
 * Component using server actions for state transitions. The shared picker
 * uses callback props (onSelect/onContinue/onSkip), not form submission.
 * This wrapper bridges the two:
 *
 *   - holds local `selectedRef` state,
 *   - emits hidden inputs the server action reads (`design_system_ref`,
 *     `action`),
 *   - calls `formAction` via `useFormStatus`/native form submit when the
 *     user picks Continue or Skip.
 *
 * Visual layout, screenshot grid, NEW badge and i18n keys all come from
 * the shared `DesignSystemPicker`. Hosts (Friendly view + wizard) now
 * render through the same component.
 */

import { useRef, useState } from "react";
import { DesignSystemPicker, type DesignSystemEntry } from "./DesignSystemPicker";

export interface DesignSystemPickerFormProps {
  catalog: DesignSystemEntry[];
  /** Hidden inputs the form submission carries through (e.g. workspace_id,
   *  new_slug, new_name) so the server action can read them after submit. */
  hiddenFields: Record<string, string>;
  /** Server action endpoint. Same shape Next.js server actions consume. */
  formAction: (formData: FormData) => void | Promise<void>;
  loadError?: boolean;
  showNewBadge?: boolean;
}

export function DesignSystemPickerForm(props: DesignSystemPickerFormProps) {
  const { catalog, hiddenFields, formAction, loadError, showNewBadge } = props;
  const formRef = useRef<HTMLFormElement | null>(null);
  const actionRef = useRef<HTMLInputElement | null>(null);
  const refRef = useRef<HTMLInputElement | null>(null);

  // Default selection: first built-in template. Matches the wizard's prior
  // behavior (the picker focused desing-system-1 as the forge default).
  const initialRef = (() => {
    const firstBuiltIn = catalog.find((e) => e.built_in_template);
    if (firstBuiltIn) return `${firstBuiltIn.asset_id}@${firstBuiltIn.version}`;
    const first = catalog[0];
    return first ? `${first.asset_id}@${first.version}` : null;
  })();
  const [selectedRef, setSelectedRef] = useState<string | null>(initialRef);

  function submit(action: "continue" | "skip") {
    if (actionRef.current) actionRef.current.value = action;
    if (refRef.current) refRef.current.value = action === "skip" ? "" : selectedRef ?? "";
    formRef.current?.requestSubmit();
  }

  return (
    <form ref={formRef} action={formAction} className="space-y-4">
      {Object.entries(hiddenFields).map(([name, value]) => (
        <input key={name} type="hidden" name={name} value={value} />
      ))}
      {/* Mutated by submit() so the server action sees the picker's choice. */}
      <input ref={actionRef} type="hidden" name="action" defaultValue="continue" />
      <input ref={refRef} type="hidden" name="design_system_ref" defaultValue={initialRef ?? ""} />
      <DesignSystemPicker
        catalog={catalog}
        selectedRef={selectedRef}
        onSelect={(ref) => setSelectedRef(ref)}
        onContinue={() => submit("continue")}
        onSkip={() => submit("skip")}
        loadError={loadError}
        showNewBadge={showNewBadge}
      />
    </form>
  );
}

export default DesignSystemPickerForm;
