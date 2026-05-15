"use client";

import { useState } from "react";
import Link from "next/link";
import { Button } from "@/components/primitives";
import { OwnersMultiSelect } from "@/components/forms/OwnersMultiSelect";
import { ScopeSelect } from "@/components/scope/ScopeSelect";
import { createWorkspace } from "./actions";

export function NewWorkspaceForm() {
  const [businessUnitId, setBusinessUnitId] = useState("");
  const [owners, setOwners] = useState<string[]>([]);

  return (
    <form
      action={createWorkspace}
      style={{ padding: 16, display: "flex", flexDirection: "column", gap: 14 }}
    >
      <label className="grid gap-1 text-sm">
        <span style={{ fontWeight: 500 }}>Business Unit</span>
        <ScopeSelect
          kind="business-unit"
          name="business_unit_id"
          required
          className="top-search"
          style={{ height: 36 }}
          value={businessUnitId}
          onChange={setBusinessUnitId}
        />
      </label>
      <label className="grid gap-1 text-sm">
        <span style={{ fontWeight: 500 }}>Workspace name</span>
        <input name="name" required className="top-search" style={{ height: 36 }} />
      </label>
      <label className="grid gap-1 text-sm">
        <span style={{ fontWeight: 500 }}>Description</span>
        <textarea
          name="description"
          rows={3}
          style={{
            background: "var(--bg-card)",
            border: "1px solid var(--border)",
            borderRadius: "var(--r-2)",
            padding: "8px 10px",
            color: "var(--fg)",
            fontFamily: "var(--f-sans)",
            fontSize: 13,
          }}
        />
      </label>
      <div className="grid gap-1 text-sm">
        <span style={{ fontWeight: 500 }}>Owners</span>
        <OwnersMultiSelect
          name="owners"
          businessUnitId={businessUnitId.trim()}
          value={owners}
          onChange={setOwners}
          required
          placeholder="Select or type owners…"
        />
        <span style={{ fontSize: 11, color: "var(--fg-3)" }}>
          Pick from existing members of the business unit, or type any local username or subject
          identifier and press Enter.
        </span>
      </div>
      <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
        <Button variant="primary" type="submit">
          Create workspace
        </Button>
        <Link href="/" style={{ color: "var(--fg-2)", fontSize: 13 }}>
          Cancel
        </Link>
      </div>
    </form>
  );
}
