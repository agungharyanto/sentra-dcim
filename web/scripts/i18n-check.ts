// Verifies that every locale has exactly the same namespaces, keys and interpolation variables as the reference
// locale (id). Exit code 1 on any missing/extra key. Run: pnpm i18n:check
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const REFERENCE = "id";
const root = fileURLToPath(new URL("../src/i18n/locales/", import.meta.url));

type Catalog = Map<string, string>; // "ns:a.b" -> value

function flatten(obj: unknown, prefix: string, out: Catalog, problems: string[]) {
  if (typeof obj === "string") {
    if (obj.trim() === "") problems.push(`empty value: ${prefix}`);
    out.set(prefix, obj);
    return;
  }
  if (obj === null || typeof obj !== "object" || Array.isArray(obj)) {
    problems.push(`not a string or object: ${prefix}`);
    return;
  }
  for (const [k, v] of Object.entries(obj)) {
    flatten(v, prefix.endsWith(":") ? `${prefix}${k}` : `${prefix}.${k}`, out, problems);
  }
}

function load(locale: string, problems: string[]): Catalog {
  const out: Catalog = new Map();
  for (const file of readdirSync(join(root, locale)).filter((f) => f.endsWith(".json")).sort()) {
    const ns = file.replace(/\.json$/, "");
    flatten(JSON.parse(readFileSync(join(root, locale, file), "utf8")), `${ns}:`, out, problems);
  }
  return out;
}

const vars = (s: string) => [...s.matchAll(/\{\{\s*(\w+)/g)].map((m) => m[1]).sort().join(",");

const problems: string[] = [];
const locales = readdirSync(root, { withFileTypes: true })
  .filter((d) => d.isDirectory())
  .map((d) => d.name)
  .sort();
if (!locales.includes(REFERENCE)) problems.push(`reference locale "${REFERENCE}" missing`);

const ref = load(REFERENCE, problems);
for (const locale of locales.filter((l) => l !== REFERENCE)) {
  const cat = load(locale, problems);
  for (const [key, value] of ref) {
    const other = cat.get(key);
    if (other === undefined) problems.push(`${locale}: missing ${key}`);
    else if (vars(value) !== vars(other)) problems.push(`${locale}: interpolation mismatch in ${key}`);
  }
  for (const key of cat.keys()) {
    if (!ref.has(key)) problems.push(`${locale}: extra ${key} (not in ${REFERENCE})`);
  }
}

if (problems.length > 0) {
  console.error(`i18n:check FAILED (${problems.length})\n- ${problems.join("\n- ")}`);
  process.exit(1);
}
console.log(`i18n:check OK: ${locales.join(", ")} identical (${ref.size} keys)`);
