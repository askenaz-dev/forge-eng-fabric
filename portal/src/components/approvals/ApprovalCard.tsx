"use client";

import { Badge } from "../primitives/Badge";
import { Button } from "../primitives/Button";
import { Check, Clock } from "../icons";
import { useLang } from "../providers/LangProvider";

export type ApprovalCriticality = "low" | "medium" | "high";

export type Approval = {
  id: string;
  agent: string;
  tag: string;
  title: string;
  meta: string;
  summary: string;
  delta_add?: number;
  delta_rem?: number;
  expires: string;
  criticality: ApprovalCriticality;
};

export type ApprovalDecision = "approve" | "review" | "reject";

export function ApprovalCard({
  approval,
  onDecide,
}: {
  approval: Approval;
  onDecide?: (id: string, decision: ApprovalDecision) => void;
}) {
  const { t } = useLang();
  return (
    <div className="approval">
      <div className="approval-head">
        <div className="ai">{approval.tag}</div>
        <div style={{ minWidth: 0, flex: 1 }}>
          <div className="title">{approval.title}</div>
          <div className="meta">{approval.meta}</div>
        </div>
        {approval.criticality === "high" && (
          <Badge tone="err" dot>
            high
          </Badge>
        )}
      </div>
      <div className="approval-summary">
        {approval.delta_add != null && approval.delta_rem != null && (
          <>
            <span className="add">+{approval.delta_add}</span>
            {" / "}
            <span className="rem">−{approval.delta_rem}</span>
            {" · "}
          </>
        )}
        {approval.summary}
      </div>
      <div className="approval-actions">
        <Button
          variant="primary"
          size="xs"
          onClick={() => onDecide?.(approval.id, "approve")}
          leading={<Check />}
        >
          {t("apr_approve")}
        </Button>
        <Button variant="secondary" size="xs" onClick={() => onDecide?.(approval.id, "review")}>
          {t("apr_review")}
        </Button>
        <Button variant="ghost" size="xs" onClick={() => onDecide?.(approval.id, "reject")}>
          {t("apr_reject")}
        </Button>
        <span className="timer">
          <Clock /> {t("apr_expires_in")} {approval.expires}
        </span>
      </div>
    </div>
  );
}
