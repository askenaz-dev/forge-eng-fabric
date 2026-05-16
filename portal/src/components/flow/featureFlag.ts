// Feature flag for the React Flow canvas introduced by the ai-flow-authoring
// change. The flag is read on the server during the editor page render and
// shipped to the client via a data attribute on the editor shell.
//
// Default is OFF during rollout (Phase A through E); the cutover in §13
// flips this to ON in dev/staging configs.

export const AI_FLOWS_CANVAS_FLAG = "AI_FLOWS_CANVAS_ENABLED";

export function isCanvasEnabledFromEnv(): boolean {
  if (typeof process === "undefined") return false;
  const v = process.env[AI_FLOWS_CANVAS_FLAG];
  if (!v) return false;
  return v === "1" || v.toLowerCase() === "true" || v.toLowerCase() === "on";
}
