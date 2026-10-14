import AxeBuilder from "@axe-core/playwright";
import { expect, type Page, test } from "@playwright/test";

import { artifactsDir, catalog, watchProblems } from "./helpers";

const tx = catalog("id", "settings");
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

async function me(page: Page) {
  return (await (await page.request.get("/api/v1/auth/me")).json()) as {
    user: { permissions: string[] };
    regional: { language: string; timezone: string; time_format: string; date_format: string };
  };
}

test.describe("settings & preferences", () => {
  // Read-only: safe against production (nothing is saved).
  test("Regional settings, Preferences and the date picker, desktop + mobile, axe", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/dashboard");
    const m = await me(page);
    expect(m.regional.timezone).toBeTruthy();

    await page.goto("/account/preferences");
    await expect(page.getByTestId("preferences-page")).toBeVisible();
    await expect(page.getByTestId("regional-preview")).toBeVisible();
    await expectAccessible(page, "preferences");
    await page.screenshot({ path: `${artifactsDir}/settings-preferences.png`, fullPage: true });

    if (m.user.permissions.includes("settings.manage")) {
      await page.goto("/settings");
      await expect(page).toHaveURL(/\/settings\/regional$/);
      await expect(page.getByRole("heading", { level: 1 })).toHaveText(shell.sidebar.settings);
      await expect(page.getByTestId("regional-timezone")).toHaveValue(/.+/);
      await page.getByTestId("regional-time_format").selectOption("12h"); // preview only, not saved
      await expect(page.getByTestId("preview-datetime")).toContainText(/AM|PM/);
      await expectAccessible(page, "regional settings");
      await page.screenshot({ path: `${artifactsDir}/settings-regional.png`, fullPage: true });
    }

    if (m.user.permissions.includes("audit.read")) {
      await page.goto("/audit");
      await page.getByTestId("datetime-trigger").first().click();
      const pop = page.getByTestId("datetime-popover");
      await expect(pop).toBeVisible();
      await expect(pop.locator('[tabindex="0"]')).toBeFocused();
      await page.keyboard.press("ArrowRight");
      await expectAccessible(page, "date picker");
      await page.screenshot({ path: `${artifactsDir}/settings-datepicker.png` });
      await page.keyboard.press("Escape");
    }

    await page.setViewportSize(mobile);
    await page.goto("/account/preferences");
    await expect(page.getByTestId("preferences-page")).toBeVisible();
    expect(await noHorizontalScroll(page)).toBe(true);
    await page.screenshot({ path: `${artifactsDir}/settings-preferences-mobile.png`, fullPage: true });
    if (m.user.permissions.includes("settings.manage")) {
      await page.goto("/settings/regional");
      await expect(page.getByTestId("settings-regional")).toBeVisible();
      expect(await noHorizontalScroll(page)).toBe(true);
      await expectAccessible(page, "regional settings mobile");
      await page.screenshot({ path: `${artifactsDir}/settings-regional-mobile.png`, fullPage: true });
    }
    expect(problems).toEqual([]);
  });

  // Changes the system settings and the account's preferences: CI throwaway stack only. Restores both at the end.
  test("system change and own override drive displayed times; audited (CI only)", async ({ page }) => {
    test.skip(process.env.E2E_MUTATIONS !== "1", "changes settings: runs only on the CI throwaway stack");
    const problems = await watchProblems(page);
    await page.goto("/settings/regional");
    await page.getByTestId("regional-timezone").selectOption("UTC");
    await page.getByTestId("regional-date_format").selectOption("YYYY-MM-DD");
    await page.getByTestId("settings-save").click();
    await expect(page.getByText(tx.regional.saved)).toBeVisible();
    await expect(page.getByTestId("reset-timezone")).toBeVisible();
    await page.screenshot({ path: `${artifactsDir}/settings-regional-saved.png`, fullPage: true });
    await expect.poll(async () => (await me(page)).regional.timezone).toBe("UTC");

    // Own preference overrides the system zone; the header clock follows it.
    await page.goto("/account/preferences");
    await page.getByTestId("regional-timezone").selectOption("Asia/Makassar");
    await page.getByTestId("regional-time_format").selectOption("12h");
    await page.getByTestId("preferences-save").click();
    await expect(page.getByText(tx.preferences.saved)).toBeVisible();
    await expect(page.getByTestId("clock")).toContainText(/(AM|PM) WITA/);
    await expect(page.getByTestId("clock")).toContainText(/\d{4}-\d{2}-\d{2}/); // date format still from the system
    await page.screenshot({ path: `${artifactsDir}/settings-preferences-saved.png`, fullPage: true });

    // Both changes are in the audit log with old → new.
    await page.goto("/audit");
    await page.getByTestId("audit-action").selectOption("settings");
    await page.getByTestId("audit-apply").click();
    await expect(page.locator('[data-testid="audit-row"][data-action="settings.update"]').first()).toBeVisible();

    // Restore: preferences follow the system, system back to defaults.
    await page.goto("/account/preferences");
    await page.getByTestId("regional-timezone").selectOption("");
    await page.getByTestId("regional-time_format").selectOption("");
    await page.getByTestId("preferences-save").click();
    await expect(page.getByText(tx.preferences.saved)).toBeVisible();
    await page.goto("/settings/regional");
    await page.getByTestId("reset-timezone").click();
    await expect(page.getByTestId("reset-timezone")).toHaveCount(0);
    await page.getByTestId("reset-date_format").click();
    await expect(page.getByTestId("reset-date_format")).toHaveCount(0);
    expect(problems).toEqual([]);
  });
});
