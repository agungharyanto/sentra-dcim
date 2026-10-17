import AxeBuilder from "@axe-core/playwright";
import { expect, type Page, test } from "@playwright/test";

import { artifactsDir, catalog, watchProblems } from "./helpers";

const tx = catalog("id", "facility");
const en = catalog("en", "facility");
const common = { id: catalog("id", "common"), en: catalog("en", "common") };
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

async function fill(page: Page, label: string, value: string) {
  const dialog = page.getByTestId("facility-dialog");
  await dialog.getByLabel(label, { exact: true }).fill(value);
}

async function save(page: Page) {
  await page.getByTestId("facility-dialog").getByTestId("facility-save").click();
  await expect(page.getByTestId("facility-dialog")).toBeHidden();
}

test.describe("facility (v0.4.1)", () => {
  // Read-only: the hierarchy and the rack list render (with or without data), desktop + mobile, axe.
  test("hierarchy and rack list render, axe, mobile", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/facility");
    await expect(page).toHaveURL(/\/facility\/hierarchy/);
    await expect(page.getByTestId("facility-hierarchy")).toBeVisible();
    await expectAccessible(page, "facility hierarchy");
    await page.goto("/facility/racks");
    await expect(page.getByTestId("rack-list")).toBeVisible();
    await expectAccessible(page, "rack list");
    await page.setViewportSize(mobile);
    await page.goto("/facility/hierarchy");
    await expect(page.getByTestId("facility-hierarchy")).toBeVisible();
    expect(await noHorizontalScroll(page)).toBe(true);
    expect(problems).toEqual([]);
  });

  // Creates a site down to racks, renames and archives: CI throwaway stack only.
  test("build site › building › floor › MMR room › rack, rename, conflict, ID ↔ EN, screenshots", async ({ page, browser }) => {
    test.skip(process.env.E2E_MUTATIONS !== "1", "creates facility data: runs only on the CI throwaway stack");
    const problems = await watchProblems(page);
    const code = `E2E${Date.now().toString(36).slice(-5).toUpperCase()}`;
    await page.goto("/facility/hierarchy");

    await page.getByTestId("facility-add-site").click();
    await fill(page, tx.fields.code, code);
    await fill(page, tx.fields.name, `Site ${code}`);
    await fill(page, tx.fields.address, "Jl. Contoh No. 1, Denpasar");
    await page.screenshot({ path: `${artifactsDir}/facility-form-site.png` });
    await save(page);
    await expect(page.getByTestId("facility-detail-name")).toHaveText(`Site ${code}`);

    const addChild = async (name: string, extra?: (p: Page) => Promise<void>) => {
      await page.getByTestId("facility-add-child").click();
      await fill(page, tx.fields.name, name);
      if (extra) await extra(page);
      await save(page);
      await expect(page.getByTestId("facility-detail-name")).toHaveText(name);
    };
    await addChild("Gedung Utama");
    await addChild("Lantai 2", (p) => p.getByTestId("facility-dialog").getByLabel(tx.fields.level).fill("2"));
    await addChild("MMR Utara", (p) => p.getByTestId("facility-dialog").getByLabel(tx.fields.room_type).selectOption("mmr"));
    await expect(page.locator('[data-testid="facility-tree"] [data-icon="mmr"]').first()).toBeVisible();

    // Invalid rack name is refused in the form; then a valid one.
    await page.getByTestId("facility-add-child").click();
    await fill(page, tx.fields.name, "-bad@name");
    await page.getByTestId("facility-dialog").getByTestId("facility-save").click();
    await expect(page.getByTestId("facility-form-error")).toHaveText(tx.hint.rackName);
    await page.getByTestId("facility-dialog").getByLabel(tx.fields.name, { exact: true }).fill("MMR-01");
    await page.getByTestId("facility-dialog").getByLabel(tx.fields.asset_tag).fill(`TAG-${code}`);
    await page.screenshot({ path: `${artifactsDir}/facility-form-rack.png` });
    await save(page);
    await expect(page.getByTestId("facility-detail-name")).toHaveText("MMR-01");
    await expectAccessible(page, "rack detail");
    await page.screenshot({ path: `${artifactsDir}/facility-tree-detail.png`, fullPage: true });

    // Rename.
    await page.getByTestId("facility-rename").click();
    await page.getByTestId("facility-dialog").getByLabel(tx.fields.name).fill("MMR-01A");
    await page.screenshot({ path: `${artifactsDir}/facility-rename.png` });
    await save(page);
    await expect(page.getByTestId("facility-detail-name")).toHaveText("MMR-01A");

    // A second tab changes the rack first: this tab's save gets 412 and offers a reload.
    const url = page.url();
    await page.getByTestId("facility-edit").click();
    const other = await browser.newPage({ storageState: await page.context().storageState() });
    await other.goto(url);
    await other.getByTestId("facility-rename").click();
    await other.getByTestId("facility-dialog").getByLabel(tx.fields.name).fill("MMR-01B");
    await other.getByTestId("facility-dialog").getByTestId("facility-save").click();
    await expect(other.getByTestId("facility-detail-name")).toHaveText("MMR-01B");
    await other.close();
    await page.getByTestId("facility-dialog").getByLabel(tx.fields.notes).fill("perubahan basi");
    await page.getByTestId("facility-dialog").getByTestId("facility-save").click();
    await expect(page.getByTestId("facility-conflict")).toContainText(tx.conflict.title);
    await page.screenshot({ path: `${artifactsDir}/facility-conflict.png` });
    await page.getByTestId("facility-reload").click();
    await expect(page.getByTestId("facility-detail-name")).toHaveText("MMR-01B");

    // Room with a rack cannot be archived.
    await page.locator('[data-testid="facility-tree"] [role=treeitem]', { hasText: "MMR Utara" }).click();
    await page.getByTestId("facility-archive").click();
    await page.getByTestId("facility-archive-confirm").click();
    await expect(page.getByTestId("facility-archive-error")).toBeVisible();
    await page.keyboard.press("Escape");

    // Rack list with the MMR icon and status badge; English.
    await page.goto("/facility/racks");
    await page.getByTestId("rack-search").fill("MMR-01B");
    await expect(page.getByTestId("rack-row").first()).toContainText("MMR-01B");
    await page.screenshot({ path: `${artifactsDir}/facility-racks.png`, fullPage: true });
    await page.getByRole("button", { name: new RegExp(common.en.language.en) }).click();
    await expect(page.getByRole("columnheader", { name: en.list.location })).toBeVisible();
    await page.screenshot({ path: `${artifactsDir}/facility-racks-en.png`, fullPage: true });
    await page.goto(url);
    await expect(page.getByRole("button", { name: en.detail.rename })).toBeVisible();
    await page.screenshot({ path: `${artifactsDir}/facility-detail-en.png`, fullPage: true });
    await page.getByRole("button", { name: new RegExp(common.id.language.id) }).click();
    await expect(page.getByRole("button", { name: tx.detail.rename })).toBeVisible();

    await page.setViewportSize(mobile);
    await page.goto(url);
    await expect(page.getByTestId("facility-detail-name")).toHaveText("MMR-01B");
    expect(await noHorizontalScroll(page)).toBe(true);
    await page.screenshot({ path: `${artifactsDir}/facility-mobile.png`, fullPage: true });
    // The 412 (stale save) and 409 (room with racks) are the expected refusals; anything else is a problem.
    expect(problems.filter((p) => !p.includes("status of 412") && !p.includes("status of 409"))).toEqual([]);
  });
});
