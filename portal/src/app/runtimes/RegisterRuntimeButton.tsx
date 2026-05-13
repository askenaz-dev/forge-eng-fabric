"use client";

import { useState } from "react";
import { Button } from "@/components/primitives";
import { RegisterRuntimeDrawer } from "./RegisterRuntimeDrawer";

export function RegisterRuntimeButton() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button variant="primary" onClick={() => setOpen(true)}>
        Register BYO runtime
      </Button>
      <RegisterRuntimeDrawer open={open} onOpenChange={setOpen} />
    </>
  );
}
