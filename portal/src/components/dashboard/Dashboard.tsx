"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Greeting } from "./Greeting";
import { DashboardHeadline } from "./Headline";
import { KpiGrid } from "./KpiGrid";
import { RunsPanel } from "./RunsPanel";
import { ApprovalsPanel } from "./ApprovalsPanel";
import { ActivityPanel } from "./ActivityPanel";
import { ServicesMeshPanel } from "./ServicesMeshPanel";
import { RunSheet } from "./RunSheet";
import type { Run } from "@/components/runs/RunRow";

export function Dashboard() {
  const router = useRouter();
  const params = useSearchParams();
  const runFromUrl = params?.get("run") ?? null;
  const [runId, setRunId] = useState<string | null>(runFromUrl);
  const [counts, setCounts] = useState<{ agents: number; approvals: number }>({ agents: 0, approvals: 0 });

  useEffect(() => {
    fetch("/api/sidebar/counts", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: { agents: number; approvals: number } | null) => {
        if (data) setCounts({ agents: data.agents, approvals: data.approvals });
      })
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    setRunId(runFromUrl);
  }, [runFromUrl]);

  const openRun = useCallback(
    (run: Run) => {
      router.replace(`/?run=${encodeURIComponent(run.id)}`, { scroll: false });
      setRunId(run.id);
    },
    [router],
  );

  const closeRun = useCallback(() => {
    router.replace("/", { scroll: false });
    setRunId(null);
  }, [router]);

  const memoised = useMemo(
    () => (
      <>
        <Greeting />
        <DashboardHeadline agentsActive={counts.agents} approvalsPending={counts.approvals} />
        <KpiGrid />
        <div className="two-col">
          <div className="stack">
            <RunsPanel onOpen={openRun} />
            <ActivityPanel />
          </div>
          <div className="stack">
            <ApprovalsPanel />
            <ServicesMeshPanel />
          </div>
        </div>
        <RunSheet id={runId} onClose={closeRun} />
      </>
    ),
    [counts.agents, counts.approvals, openRun, runId, closeRun],
  );

  return memoised;
}
