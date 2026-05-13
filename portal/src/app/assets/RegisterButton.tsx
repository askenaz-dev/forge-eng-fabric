"use client";

import { useState } from "react";
import { Button } from "@/components/primitives";
import { RegisterDrawer, type AssetKind } from "./RegisterDrawer";

export function RegisterButton({ workspaceId, lockedKind }: { workspaceId: string; lockedKind?: AssetKind }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button variant="primary" onClick={() => setOpen(true)} disabled={!workspaceId} title={!workspaceId ? "Load a workspace first" : undefined}>
        Register asset
      </Button>
      <RegisterDrawer open={open} onOpenChange={setOpen} workspaceId={workspaceId} lockedKind={lockedKind} />
    </>
  );
}
