"use client";

import { useState } from "react";
import { Button } from "@/components/primitives";
import { useToast } from "@/components/providers/ToastProvider";

export function RevokeButton({ tokenId }: { tokenId: string }) {
  const toast = useToast();
  const [busy, setBusy] = useState(false);

  async function revoke() {
    if (!confirm("Revoke this token? All requests bearing it will be refused within 5 seconds.")) return;
    setBusy(true);
    try {
      const resp = await fetch(`/api/gateway/tokens/${encodeURIComponent(tokenId)}`, { method: "DELETE" });
      if (!resp.ok) throw new Error(`gateway ${resp.status}`);
      toast.success("Token revoked");
      setTimeout(() => window.location.reload(), 600);
    } catch (e) {
      toast.err(e instanceof Error ? e.message : "revoke failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Button variant="danger" size="xs" onClick={revoke} disabled={busy}>
      {busy ? "Revoking…" : "Revoke"}
    </Button>
  );
}
