"use client";

import { useState } from "react";
import { Button } from "@/components/primitives";
import { RegisterDrawer, type AssetKind } from "./RegisterDrawer";
import { RegisterMCPDrawer } from "./RegisterMCPDrawer";
import { RegisterSkillDrawer } from "./RegisterSkillDrawer";

export function RegisterButton({ workspaceId, lockedKind }: { workspaceId: string; lockedKind?: AssetKind }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button variant="primary" onClick={() => setOpen(true)}>
        Register asset
      </Button>
      {lockedKind === "mcp" ? (
        <RegisterMCPDrawer open={open} onOpenChange={setOpen} workspaceId={workspaceId} />
      ) : lockedKind === "skill" ? (
        <RegisterSkillDrawer open={open} onOpenChange={setOpen} workspaceId={workspaceId} />
      ) : (
        <RegisterDrawer open={open} onOpenChange={setOpen} workspaceId={workspaceId} lockedKind={lockedKind} />
      )}
    </>
  );
}
