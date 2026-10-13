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

export const user = process.env.SENTRA_WEB_USER ?? "";
export const pass = process.env.SENTRA_WEB_PASS ?? "";

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
