import { createHmac } from "node:crypto";
import { readFileSync } from "node:fs";

import type { Page } from "@playwright/test";

/** Reads a translation catalog from the web app, so expectations follow translation changes. */
export function catalog(lng: string, ns: string) {
  return JSON.parse(readFileSync(new URL(`../../web/src/i18n/locales/${lng}/${ns}.json`, import.meta.url), "utf8"));
}

/** Backend menu registry fixture (kept in sync by `go test ./internal/ui/service`). */
export function registry(): {
  menu_groups: { key: string; order: number }[];
  menus: { key: string; path: string; status: string; group: string }[];
} {
  return JSON.parse(readFileSync(new URL("../../web/src/test/fixtures/ui-registry.json", import.meta.url), "utf8"));
}


/** Application account (session login, v0.3.1): an active account that already changed its first password. */
export const appUser = process.env.SENTRA_APP_USER ?? "";
export const appPass = process.env.SENTRA_APP_PASS ?? "";

/** Signed-in browser state written by global-setup.ts (session + CSRF cookies), used by every spec. */
export const stateFile = "/tmp/e2e-state.json";

/** Screenshots go to E2E_ARTIFACTS (uploaded by CI, never committed). */
export const artifactsDir = process.env.E2E_ARTIFACTS ?? "/tmp/e2e-artifacts";

/** Collects console errors/warnings, uncaught exceptions and CSP violations for the page lifetime. */
export async function watchProblems(page: Page) {
  const problems: string[] = [];
  page.on("console", (msg) => {
    if (msg.type() === "error" || msg.type() === "warning") problems.push(`console.${msg.type()}: ${msg.text()}`);
  });
  page.on("pageerror", (err) => problems.push(`pageerror: ${err.message}`));
  await page.addInitScript(() => {
    document.addEventListener("securitypolicyviolation", (e) =>
      console.error(`CSP violation: ${e.violatedDirective} blocked ${e.blockedURI || "inline"}`),
    );
  });
  return problems;
}

/**
 * MFA for the test account (v0.3.3): SENTRA_APP_TOTP_SECRET exists only for the CI throwaway account; against
 * production run.sh passes ONE code typed by the owner (SENTRA_APP_TOTP), used by global-setup.ts.
 */
export const appTotpSecret = process.env.SENTRA_APP_TOTP_SECRET ?? "";
export const appTotpCode = process.env.SENTRA_APP_TOTP ?? "";

/**
 * Waits for the next 30 s step and returns its code: every sign-in of the CI account gets a step no earlier sign-in
 * used (the server refuses replayed steps).
 */
export async function freshTotp(secret: string): Promise<string> {
  await new Promise((r) => setTimeout(r, 30000 - (Date.now() % 30000) + 200));
  return totp(secret);
}

/** RFC 6238 TOTP (SHA1, 6 digits, 30 s) of a base32 secret; offset in 30 s steps (anti-replay: use a new step). */
export function totp(secret: string, offset = 0): string {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = "";
  for (const ch of secret.replace(/[\s=]/g, "").toUpperCase()) bits += alphabet.indexOf(ch).toString(2).padStart(5, "0");
  const key = Buffer.from((bits.match(/.{8}/g) ?? []).map((b) => parseInt(b, 2)));
  const step = Math.floor(Date.now() / 30000) + offset;
  const msg = Buffer.alloc(8);
  msg.writeBigUInt64BE(BigInt(step));
  const mac = createHmac("sha1", key).update(msg).digest();
  const off = (mac[mac.length - 1] ?? 0) & 0x0f;
  return String((mac.readUInt32BE(off) & 0x7fffffff) % 1_000_000).padStart(6, "0");
}
