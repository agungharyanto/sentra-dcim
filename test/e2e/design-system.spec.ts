import AxeBuilder from "@axe-core/playwright";
import { expect, type Page, test } from "@playwright/test";

import { artifactsDir, catalog, watchProblems } from "./helpers";

const id = { ds: catalog("id", "ds"), common: catalog("id", "common") };
const en = { ds: catalog("en", "ds") };

/** axe (WCAG 2.x A/AA): no serious or critical violations. */
async function expectAccessible(page: Page, label: string) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  const bad = results.violations
    .filter((v) => v.impact === "serious" || v.impact === "critical")
    .map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`);
  expect(bad, `axe violations (${label})`).toEqual([]);
}

test.describe("design system showcase", () => {

  for (const theme of ["dark", "light"] as const) {
    test(`${theme} theme: components, overlays, toast, ID↔EN, a11y, no console/CSP errors`, async ({ page }) => {
      const problems = await watchProblems(page);
      await page.addInitScript((t) => window.localStorage.setItem("sentra.theme", t), theme);

      await page.goto("/design-system");
      const h1 = page.getByRole("heading", { level: 1 });
      await expect(h1).toHaveText(id.ds.showcase.title);
      await expect(page.locator("html")).toHaveClass(theme === "dark" ? /\bdark\b/ : /^(?!.*\bdark\b)/);
      await expect(page.getByRole("button", { name: id.ds.theme[theme] })).toHaveAttribute("aria-pressed", "true");

      // Status: icon + label for all five statuses.
      const badges = page.getByTestId("status-badges");
      for (const s of ["normal", "warning", "critical", "offline", "maintenance"]) {
        const badge = badges.locator(`[data-status="${s}"]`);
        await expect(badge).toHaveText(id.ds.status[s]);
        await expect(badge.locator("svg")).toHaveCount(1);
      }

      await expectAccessible(page, `${theme} page`);
      await page.screenshot({ path: `${artifactsDir}/design-system-${theme}-id.png`, fullPage: true });

      // Dialog
      await page.getByRole("button", { name: id.ds.showcase.overlays.openDialog }).click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toContainText(id.ds.showcase.overlays.dialogTitle);
      await expectAccessible(page, `${theme} dialog`);
      await page.screenshot({ path: `${artifactsDir}/design-system-${theme}-dialog.png` });
      await dialog.getByRole("button", { name: id.common.actions.cancel }).click();
      await expect(dialog).toBeHidden();

      // Sheet
      await page.getByRole("button", { name: id.ds.showcase.overlays.openSheet }).click();
      const sheet = page.getByRole("dialog");
      await expect(sheet).toContainText(id.ds.showcase.overlays.sheetTitle);
      await page.keyboard.press("Escape");
      await expect(sheet).toBeHidden();

      // Tooltip
      await page.getByRole("button", { name: id.ds.showcase.overlays.tooltipTrigger }).hover();
      await expect(page.getByRole("tooltip")).toContainText(id.ds.showcase.overlays.tooltip);
      await page.mouse.move(0, 0);

      // Select
      await page.getByTestId("site-select").click();
      await page.getByRole("option", { name: id.ds.showcase.form.sites.drc }).click();
      await expect(page.getByTestId("site-select")).toContainText(id.ds.showcase.form.sites.drc);

      // Dropdown menu
      await page.getByRole("button", { name: id.ds.showcase.overlays.menu }).click();
      await expect(page.getByRole("menu")).toContainText(id.ds.showcase.overlays.menuItems.ack);
      await page.keyboard.press("Escape");
      await expect(page.getByRole("menu")).toBeHidden();

      // Copy button (reusable domain component)
      await expect(page.getByTestId("copy-demo").getByRole("button", { name: id.ds.showcase.copyDemo })).toBeVisible();

      // Toast
      await page.getByRole("button", { name: id.ds.showcase.feedback.showToast }).click();
      const toast = page.locator("[data-toast]");
      await expect(toast).toContainText(id.ds.showcase.feedback.toastTitle);
      await page.waitForTimeout(400); // let the enter animation finish before measuring
      await expectAccessible(page, `${theme} toast`);
      await page.screenshot({ path: `${artifactsDir}/design-system-${theme}-toast.png` });
      await expect(toast).toHaveCount(0, { timeout: 10_000 }); // auto-dismiss, so later scans see no fading toast

      // Language ID → EN
      await page.getByRole("button", { name: /English/ }).click();
      await expect(h1).toHaveText(en.ds.showcase.title);
      await expect(badges.locator('[data-status="critical"]')).toHaveText(en.ds.status.critical);
      await expectAccessible(page, `${theme} page EN`);
      await page.screenshot({ path: `${artifactsDir}/design-system-${theme}-en.png`, fullPage: true });

      expect(problems).toEqual([]);
    });
  }
});
