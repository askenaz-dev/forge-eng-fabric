#!/usr/bin/env node
// Lints Alfred-authored copy in the portal dictionary for persona compliance.
//
// Rules (per design/alfred-identity/PERSONA.md):
//   - No emoji (any character with Emoji property)
//   - No exclamation marks
//   - No first-person plural ("we/us/our/nuestro/nosotros")
//   - Every alfred.* / alfred_* key must appear in both ES and EN
//
// Exits non-zero on any violation so CI fails. The script is intentionally
// dependency-free (works under plain Node 20+).

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const DICT_PATH = resolve(__dirname, "..", "portal", "src", "i18n", "dictionary.ts");

const EMOJI_RE = /\p{Extended_Pictographic}/u;
const EXCLAMATION_RE = /!/;
const FIRST_PERSON_PLURAL_RE =
  /\b(we|us|our|ours|let's|nosotros|nuestra|nuestro|nuestras|nuestros)\b/i;

const ALFRED_KEY_RE = /^(alfred_|alfred\.)/;

function loadDictionary() {
  const src = readFileSync(DICT_PATH, "utf8");
  const result = { es: {}, en: {} };
  for (const lang of ["es", "en"]) {
    // The dictionary file is a TS const; we extract `lang: { ... }` blocks
    // with a forgiving regex. This is acceptable because the file is
    // line-oriented and stays small.
    const blockRe = new RegExp(`${lang}:\\s*{([\\s\\S]*?)}\\s*,?\\s*(?:en|es|\\})`);
    const blockMatch = src.match(blockRe);
    if (!blockMatch) continue;
    const block = blockMatch[1];
    const keyRe = /([A-Za-z0-9_]+)\s*:\s*"((?:[^"\\]|\\.)*)"\s*,/g;
    let m;
    while ((m = keyRe.exec(block))) {
      result[lang][m[1]] = m[2];
    }
  }
  return result;
}

function lint(dict) {
  const violations = [];
  const alfredKeysEs = Object.keys(dict.es).filter((k) => ALFRED_KEY_RE.test(k));
  const alfredKeysEn = Object.keys(dict.en).filter((k) => ALFRED_KEY_RE.test(k));

  for (const k of alfredKeysEs) {
    if (!(k in dict.en)) violations.push(`missing EN translation for key: ${k}`);
  }
  for (const k of alfredKeysEn) {
    if (!(k in dict.es)) violations.push(`missing ES translation for key: ${k}`);
  }
  for (const lang of ["es", "en"]) {
    for (const [k, v] of Object.entries(dict[lang])) {
      if (!ALFRED_KEY_RE.test(k)) continue;
      if (EMOJI_RE.test(v)) violations.push(`emoji in ${lang}.${k}: ${JSON.stringify(v)}`);
      if (EXCLAMATION_RE.test(v))
        violations.push(`exclamation mark in ${lang}.${k}: ${JSON.stringify(v)}`);
      if (FIRST_PERSON_PLURAL_RE.test(v))
        violations.push(`first-person plural in ${lang}.${k}: ${JSON.stringify(v)}`);
    }
  }
  return violations;
}

const dict = loadDictionary();
const issues = lint(dict);
if (issues.length > 0) {
  console.error("alfred copy lint failed:");
  for (const issue of issues) console.error(`  - ${issue}`);
  process.exit(1);
}
console.log("alfred copy lint ok");
