// i18n parity check.
//
// Reads dictionary.ts via the TypeScript compiler API would be ideal, but the
// dictionary is structured plainly enough that we can scan the raw source for
// `es:` and `en:` keys and assert exact parity.

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const sourcePath = resolve(__dirname, "dictionary.ts");
const source = readFileSync(sourcePath, "utf8");

function extractBlock(label) {
  const re = new RegExp(`${label}\\s*:\\s*\\{([\\s\\S]*?)^\\s*\\},?\\s*$`, "m");
  const match = source.match(re);
  if (!match) {
    console.error(`i18n:check: could not locate '${label}' block in dictionary.ts`);
    process.exit(2);
  }
  return match[1];
}

function extractKeys(block) {
  const keys = new Set();
  const lineRe = /^\s*([a-z_][a-z0-9_]*)\s*:\s*/gim;
  let m;
  while ((m = lineRe.exec(block))) {
    keys.add(m[1]);
  }
  return keys;
}

const esKeys = extractKeys(extractBlock("es"));
const enKeys = extractKeys(extractBlock("en"));

const missingInEn = [...esKeys].filter((k) => !enKeys.has(k));
const missingInEs = [...enKeys].filter((k) => !esKeys.has(k));

if (missingInEn.length === 0 && missingInEs.length === 0) {
  console.log(`i18n:check: parity ok (${esKeys.size} keys)`);
  process.exit(0);
}

if (missingInEn.length > 0) {
  console.error(`i18n:check: keys missing in 'en' (${missingInEn.length}):`);
  for (const k of missingInEn) console.error(`  - ${k}`);
}
if (missingInEs.length > 0) {
  console.error(`i18n:check: keys missing in 'es' (${missingInEs.length}):`);
  for (const k of missingInEs) console.error(`  - ${k}`);
}
process.exit(1);
