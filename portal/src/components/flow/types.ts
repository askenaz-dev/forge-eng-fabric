// Internal types for the React Flow canvas components.

export interface DryRunStepTrace {
  stepId: string;
  type: string;
  status: "running" | "completed" | "failed" | "waiting" | "skipped";
  durationMs?: number;
  inputs?: Record<string, unknown>;
  outputs?: Record<string, unknown>;
  failureReason?: string;
}
