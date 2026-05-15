import { Suspense } from "react";
import { AutonomousActivityView } from "./AutonomousActivityView";

export const dynamic = "force-dynamic";
export const metadata = { title: "Autonomous Activity — Alfred" };

export default function AutonomousActivityPage() {
  return (
    <Suspense fallback={<div className="p-6 text-muted-foreground">Loading…</div>}>
      <AutonomousActivityView />
    </Suspense>
  );
}
