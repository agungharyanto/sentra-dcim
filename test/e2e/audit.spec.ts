import { readFileSync } from "node:fs";

import AxeBuilder from "@axe-core/playwright";
import { expect, type Page, test } from "@playwright/test";

import { artifactsDir, catalog, watchProblems } from "./helpers";

const tx = catalog("id", "audit");
const shell = catalog("id", "shell");
const mobile = { width: 390, height: 844 };

async function expectAccessible(page: Page, label: string) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  const bad = results.violations
    .filter((v) => v.impact === "serious" || v.impact === "critical")
    .map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`);
  expect(bad, `axe violations (${label})`).toEqual([]);
}

const noHorizontalScroll = (page: Page) =>
  page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth);

async function permissions(page: Page): Promise<string[]> {
  return (await (await page.request.get("/api/v1/auth/me")).json()).user.permissions as string[];
}

test.describe("audit log", () => {
  // Read-only: safe against production (no verification, no export).
  test("list, filters, details, desktop + mobile, axe", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/dashboard");
    test.skip(!(await permissions(page)).includes("audit.read"), "test account has no audit.read");

    await page.goto("/audit");
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(shell.sidebar.audit);
    await expect(page.getByText(tx.description)).toBeVisible();
    await expect(page.getByTestId("audit-row").first()).toBeVisible();
    await expect(page.getByTestId("audit-integrity")).not.toHaveAttribute("data-state", "loading");
    await expectAccessible(page, "audit list");
    await page.screenshot({ path: `${artifactsDir}/audit-list.png`, fullPage: true });

    // Every signed-in session started with a successful login: filter for it.
    await page.getByTestId("audit-action").selectOption("auth.login.success");
    await page.getByTestId("audit-apply").click();
    const rows = page.getByTestId("audit-row");
    await expect(rows.first()).toHaveAttribute("data-action", "auth.login.success");
    for (const action of await rows.evaluateAll((els) => els.map((e) => e.getAttribute("data-action")))) {
      expect(action).toBe("auth.login.success");
    }
    await rows.first().getByRole("button").click();
    const detail = page.getByTestId("audit-detail");
    await expect(detail).toContainText("auth.login.success");
    await expectAccessible(page, "audit detail");
    await page.screenshot({ path: `${artifactsDir}/audit-detail.png` });
    await page.keyboard.press("Escape");
    await expect(detail).toHaveCount(0);

    await page.setViewportSize(mobile);
    await expect(page.getByTestId("audit-card").first()).toBeVisible();
    expect(await noHorizontalScroll(page)).toBe(true);
    await expectAccessible(page, "audit list mobile");
    await page.screenshot({ path: `${artifactsDir}/audit-list-mobile.png`, fullPage: true });
    expect(problems).toEqual([]);
  });

  test("without permission: no menu, Access denied page (permissions mocked)", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.route("**/api/v1/auth/me", async (route) => {
      const res = await route.fetch();
      const body = await res.json();
      body.user.roles = ["technician"];
      body.user.permissions = [];
      await route.fulfill({ response: res, json: body });
    });
    await page.goto("/audit");
    await expect(page.getByTestId("forbidden")).toContainText(shell.pages.forbidden.title);
    await expect(page.locator('a[data-menu="audit"]')).toHaveCount(0);
    expect(problems).toEqual([]);
  });

  // Writes audit events (verification, export, failed logins with formula payloads): CI throwaway stack only.
  test("verify now and CSV export with neutralised formulas (CI only)", async ({ page, browser }) => {
    test.skip(process.env.E2E_MUTATIONS !== "1", "writes audit events: runs only on the CI throwaway stack");
    const problems = await watchProblems(page);
    await page.goto("/audit");
    const base = new URL(page.url()).origin;
    // A separate browser context (no session cookie): failed logins with attacker-controlled User-Agents.
    const anon = await browser.newContext({ storageState: { cookies: [], origins: [] } });
    for (const ua of ["=HYPERLINK(\"http://evil\")", "@SUM(A1)"]) {
      await anon.request.post("/api/v1/auth/login", {
        headers: { "X-Sentra-CSRF": "login", Origin: base, "User-Agent": ua },
        data: { username: "-e2e.formula", password: "wrong-password-123" },
      });
    }

    await anon.close();

    await page.getByTestId("audit-verify").click();
    await expect(page.getByTestId("audit-integrity")).toHaveAttribute("data-state", "intact");
    await page.screenshot({ path: `${artifactsDir}/audit-verified.png`, fullPage: true });

    await page.getByTestId("audit-action").selectOption("auth.login.failure");
    await page.getByTestId("audit-apply").click();
    await expect(page.getByTestId("audit-row").first()).toHaveAttribute("data-action", "auth.login.failure");
    const [download] = await Promise.all([page.waitForEvent("download"), page.getByTestId("audit-export").click()]);
    expect(download.suggestedFilename()).toMatch(/^sentra-audit-\d{8}-\d{6}\.csv$/);
    const csv = readFileSync(await download.path(), "utf8");
    const lines = csv.trim().split("\n");
    expect(lines[0]).toMatch(/^seq,occurred_at_utc,occurred_at_local,/);
    expect(csv).toContain("'=HYPERLINK");
    expect(csv).toContain("'@SUM(A1)");
    expect(csv).toContain("'-e2e.formula");
    expect(csv).not.toMatch(/(^|,)"?[=+@]/m);
    expect(problems).toEqual([]);
  });
});
