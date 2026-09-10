#!/usr/bin/env node
// Every t("literal") must exist in both catalogues, and the catalogues must
// stay key-for-key identical. A missing key is invisible in review: t() falls
// back to the key itself, so the UI just renders "settings.gitlabTitle".
//   node scripts/i18n-check.mjs
import { readFileSync, globSync } from "node:fs";

const flat = (o, p = "", out = {}) => {
  for (const [k, v] of Object.entries(o)) {
    const key = p ? `${p}.${k}` : k;
    if (v && typeof v === "object") flat(v, key, out);
    else out[key] = String(v);
  }
  return out;
};

const load = (f) => flat(JSON.parse(readFileSync(`src/messages/${f}.json`, "utf8")));
const en = load("en");
const pt = load("pt-BR");

const used = new Map(); // key -> first file that uses it
for (const file of globSync("src/**/*.{ts,tsx}")) {
  const text = readFileSync(file, "utf8");
  for (const m of text.matchAll(/\bt\(\s*"([a-zA-Z][\w.]*)"/g)) {
    if (!m[1].endsWith(".") && !used.has(m[1])) used.set(m[1], file); // t("ns.x." + v) prefixes are dynamic
  }
}

const problems = [];
for (const [key, file] of used) {
  if (!(key in en)) problems.push(`missing in en.json:    ${key}  (${file})`);
  if (!(key in pt)) problems.push(`missing in pt-BR.json: ${key}  (${file})`);
}
for (const key of Object.keys(en)) if (!(key in pt)) problems.push(`parity: en has ${key}, pt-BR does not`);
for (const key of Object.keys(pt)) if (!(key in en)) problems.push(`parity: pt-BR has ${key}, en does not`);
for (const [key, v] of Object.entries(en)) {
  if (pt[key] !== undefined && pt[key] === v && /\s/.test(v) && !/^[A-Z][a-zA-Z]+( [A-Z][a-zA-Z]+)*$/.test(v)) {
    // identical multi-word values are usually an untranslated pt-BR entry
    problems.push(`untranslated?: ${key} = "${v}"`);
  }
}

const literalUnused = Object.keys(en).filter((k) => !used.has(k)).length;
console.log(`${used.size} literal keys used, ${Object.keys(en).length} in catalogue (${literalUnused} not referenced literally — dynamic keys land here too)`);
if (problems.length) {
  const hard = problems.filter((p) => !p.startsWith("untranslated?"));
  console.log(problems.join("\n"));
  console.log(`\n${problems.length} problems (${hard.length} hard)`);
  process.exit(hard.length ? 1 : 0);
}
console.log("catalogues OK");
