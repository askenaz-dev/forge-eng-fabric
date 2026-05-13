// Glossary audit: verifies that canonical domain terms render with the agreed
// ES/EN translation. The expected pairs come from openspec/changes/forge-portal-rebranding
// /specs/portal-i18n/spec.md (Requirement: ES/EN parity for all platform terms).

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const sourcePath = resolve(__dirname, "dictionary.ts");
const source = readFileSync(sourcePath, "utf8");

function literal(key, lang) {
  const blockMatch = source.match(new RegExp(`${lang}\\s*:\\s*\\{([\\s\\S]*?)^\\s*\\},?\\s*$`, "m"));
  if (!blockMatch) return null;
  const block = blockMatch[1];
  const lineRe = new RegExp(`${key}\\s*:\\s*"((?:[^"\\\\]|\\\\.)*)"`);
  const m = block.match(lineRe);
  return m ? m[1] : null;
}

const PAIRS = [
  ["nav_dashboard",   "Tablero",                 "Dashboard"],
  ["nav_approvals",   "Aprobaciones",            "Approvals"],
  ["nav_specs",       "Specs (OpenSpec)",        "Specs (OpenSpec)"],
  ["nav_policies",    "Políticas (OPA)",         "Policies (OPA)"],
  ["nav_audit",       "Auditoría",               "Audit"],
  ["nav_obs",         "Métricas y trazas",       "Metrics & traces"],
  ["svc_title",       "Mesh de servicios",       "Service mesh"],
  ["apr_sub",         "Human-in-the-loop · OPA", "Human-in-the-loop · OPA"],
  ["apr_title",       "Cola de aprobación",      "Approval queue"],
  ["kpi_runs",        "Runs en curso",           "Runs in flight"],
  ["kpi_success",     "Éxito 24 h",              "Success 24 h"],
  ["kpi_p95",         "p95 latencia",            "p95 latency"],
  ["kpi_savings",     "Horas ahorradas / semana","Hours saved / week"],
  ["h_new_run",       "Lanzar workflow",         "Launch workflow"],
  ["h_invite",        "Invitar equipo",          "Invite team"],
];

let failed = 0;
for (const [key, expectedEs, expectedEn] of PAIRS) {
  const actualEs = literal(key, "es");
  const actualEn = literal(key, "en");
  if (actualEs !== expectedEs) {
    console.error(`glossary: '${key}' ES expected "${expectedEs}" got "${actualEs}"`);
    failed++;
  }
  if (actualEn !== expectedEn) {
    console.error(`glossary: '${key}' EN expected "${expectedEn}" got "${actualEn}"`);
    failed++;
  }
}

if (failed > 0) {
  console.error(`glossary-audit: ${failed} discrepancies`);
  process.exit(1);
}

console.log(`glossary-audit: ${PAIRS.length} terms canonical`);
