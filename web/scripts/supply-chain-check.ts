// Fails when the pnpm minimumReleaseAge supply-chain policy is bypassed (ISS-031): any exclusion list, or the
// release age lowered below the default 24h, in pnpm-workspace.yaml, .npmrc or package.json. Run: pnpm supply:check
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

/** pnpm's default window (minutes); the policy may be made stricter, never looser. */
export const MIN_RELEASE_AGE_MINUTES = 1440;

export const POLICY_FILES = ["pnpm-workspace.yaml", ".npmrc", "package.json"] as const;

// Key spellings across YAML (camelCase), .npmrc (kebab-case) and JSON (quoted, possibly mid-line). A key must start
// a line or follow whitespace, a quote, "{" or ","; prose in comments has no ":"/"=" after the key and never matches.
const key = (name: string) => `(?:^|[\\s{,"'])${name}["']?\\s*[:=]`;
const EXCLUDE = new RegExp(key("minimum[-_]?release[-_]?age[-_]?(?:exclude|ignore)[\\w-]*"), "im");
const AGE = new RegExp(`${key("minimum[-_]?release[-_]?age")}\\s*["']?([^\\s,"'#}]*)`, "gim");

/** Returns human-readable violations for the given file contents (name → text). */
export function findPolicyViolations(files: Record<string, string>): string[] {
  const out: string[] = [];
  for (const [name, text] of Object.entries(files)) {
    if (EXCLUDE.test(text)) out.push(`${name}: minimumReleaseAge exclusion list is forbidden`);
    for (const m of text.matchAll(AGE)) {
      const minutes = Number(m[1]);
      if (!Number.isFinite(minutes) || m[1] === "" || minutes < MIN_RELEASE_AGE_MINUTES) {
        out.push(`${name}: minimumReleaseAge "${m[1]}" is below ${MIN_RELEASE_AGE_MINUTES} minutes`);
      }
    }
  }
  return out;
}

function main() {
  const root = fileURLToPath(new URL("..", import.meta.url));
  const files: Record<string, string> = {};
  for (const f of POLICY_FILES) {
    const p = join(root, f);
    if (existsSync(p)) files[f] = readFileSync(p, "utf8");
  }
  const violations = findPolicyViolations(files);
  if (violations.length > 0) {
    console.error(
      `supply:check FAILED — pnpm release-age policy bypassed (ISS-031)\n- ${violations.join("\n- ")}\n` +
        "Pin the newest version older than the policy window instead of excluding fresh releases.",
    );
    process.exit(1);
  }
  console.log(`supply:check OK: no minimumReleaseAge bypass in ${Object.keys(files).join(", ")}`);
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) main();
