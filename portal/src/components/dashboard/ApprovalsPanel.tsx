"use client";

import { useCallback, useEffect, useState } from "react";
import { Card, CardHeader, Badge } from "@/components/primitives";
import { useLang } from "@/components/providers/LangProvider";
import { useToast } from "@/components/providers/ToastProvider";
import {
  ApprovalCard,
  type Approval,
  type ApprovalDecision,
} from "@/components/approvals/ApprovalCard";
import { useSSE } from "./useSSE";

export function ApprovalsPanel() {
  const { t } = useLang();
  const toast = useToast();
  const [approvals, setApprovals] = useState<Approval[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    setError(null);
    fetch("/api/approvals?status=pending&limit=10", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(`status ${r.status}`))))
      .then((data: { approvals: Approval[] }) => setApprovals(data.approvals ?? []))
      .catch((err) => setError((err as Error).message));
  }, []);

  useEffect(load, [load]);

  // Live updates: incoming requests prepend; decisions invalidate.
  useSSE(["approvals.requested.v1", "approvals.granted.v1", "approvals.denied.v1"], () => load());

  const decide = useCallback(
    async (id: string, decision: ApprovalDecision) => {
      if (decision === "review") {
        // Navigate handled by parent.
        return;
      }
      try {
        const r = await fetch(`/api/approvals/${id}/decisions`, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ decision }),
        });
        if (!r.ok) throw new Error(await r.text());
        toast.success(decision === "approve" ? t("toast_approved") : t("toast_rejected"));
        setApprovals((cur) => cur?.filter((a) => a.id !== id) ?? null);
      } catch (err) {
        toast.err((err as Error).message);
      }
    },
    [t, toast],
  );

  return (
    <Card>
      <CardHeader
        title={t("apr_title")}
        sub={t("apr_sub")}
        right={
          approvals && (
            <Badge tone="ember" dot>
              {approvals.length}
            </Badge>
          )
        }
      />
      {approvals == null && !error && (
        <div style={{ padding: 14 }}>
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="approval">
              <div className="skeleton" style={{ height: 14, width: "60%", marginBottom: 8 }} />
              <div className="skeleton" style={{ height: 30 }} />
            </div>
          ))}
        </div>
      )}
      {error && (
        <div className="note" style={{ padding: "32px 18px", textAlign: "center" }}>
          {error}
        </div>
      )}
      {approvals?.length === 0 && (
        <div className="note" style={{ padding: "32px 18px", textAlign: "center" }}>
          {t("apr_no_items")}
        </div>
      )}
      {approvals?.map((a) => (
        <ApprovalCard key={a.id} approval={a} onDecide={decide} />
      ))}
    </Card>
  );
}
