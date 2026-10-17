import AxeBuilder from "@axe-core/playwright";
import { expect, type Page, test } from "@playwright/test";

import { artifactsDir, catalog, registry, watchProblems } from "./helpers";

const cat = { id: catalog("id", "shell"), en: catalog("en", "shell") };
const { menus } = registry();

async function expectAccessible(page: Page, label: string) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  const bad = results.violations
    .filter((v) => v.impact === "serious" || v.impact === "critical")
    .map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`);
  expect(bad, `axe violations (${label})`).toEqual([]);
}

/** Expected page for a registry menu. */
function pageTestId(status: string, key: string) {
  if (key === "dashboard") return "dashboard-empty";
  if (key === "help") return "help-center";
  if (key === "users") return "users-page"; // the test account has users.read (Admin)
  if (key === "audit") return "audit-page"; // and audit.read (Admin)
  if (key === "settings") return "settings-regional"; // and settings.manage (Admin); /settings opens Regional
  if (key === "facility") return "facility-hierarchy"; // facility.read (every role); /facility opens the hierarchy
  return status === "not_connected" ? "not-connected" : "coming-soon";
}

const mainNav = (page: Page) => page.getByRole("navigation", { name: cat.id.layout.mainNav });

test.describe("app shell", () => {

  test("clicks all 15 menus: URL, aria-current, title and standard page", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/dashboard");
    expect(menus).toHaveLength(15);
    for (const m of menus) {
      const link = mainNav(page).locator(`a[data-menu="${m.key}"]`);
      await link.click();
      // Modules with sub-pages open their first one (/settings/regional, /facility/hierarchy).
      await expect(page).toHaveURL(new RegExp(`${m.path}(/[a-z-]+)?$`));
      await expect(link).toHaveAttribute("aria-current", "page");
      await expect(mainNav(page).locator('a[aria-current="page"]')).toHaveCount(1);
      await expect(page.getByRole("heading", { level: 1 })).toHaveText(cat.id.sidebar[m.key]);
      await expect(page.getByTestId(pageTestId(m.status, m.key))).toBeVisible();
    }
    expect(problems).toEqual([]);
  });

  test("deep links straight into every route, and 404", async ({ page }) => {
    const problems = await watchProblems(page);
    for (const m of menus) {
      const res = await page.goto(m.path);
      expect(res?.status(), m.path).toBe(200);
      await expect(page.getByRole("heading", { level: 1 })).toHaveText(cat.id.sidebar[m.key]);
      await expect(page.getByTestId(pageTestId(m.status, m.key))).toBeVisible();
    }
    await page.goto("/rak/tidak/ada");
    await expect(page.getByTestId("not-found")).toContainText("/rak/tidak/ada");
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(cat.id.pages.notFound.title);
    expect(problems).toEqual([]);
  });

  test("collapsed sidebar is remembered after reload and shows tooltips", async ({ page }) => {
    await page.goto("/dashboard");
    await page.getByRole("button", { name: cat.id.layout.collapse }).click();
    await expect(page.locator("#app-sidebar")).toHaveAttribute("data-collapsed", "true");
    await page.reload();
    await expect(page.locator("#app-sidebar")).toHaveAttribute("data-collapsed", "true");
    await mainNav(page).locator('a[data-menu="assets"]').hover();
    await expect(page.getByRole("tooltip")).toContainText(cat.id.sidebar.assets);
    await expectAccessible(page, "collapsed");
    await page.screenshot({ path: `${artifactsDir}/shell-collapsed.png` });
    await page.getByRole("button", { name: cat.id.layout.expand }).click();
    await expect(page.locator("#app-sidebar")).toHaveAttribute("data-collapsed", "false");
  });

  test("keyboard: skip link first, then Tab reaches the menu", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(cat.id.sidebar.dashboard); // shell rendered
    await page.keyboard.press("Tab");
    const skip = page.getByRole("link", { name: cat.id.layout.skipToContent });
    await expect(skip).toBeFocused();
    await expect(skip).toBeVisible();
    await page.keyboard.press("Tab");
    await expect(page.locator(':focus[data-menu="dashboard"]')).toHaveCount(1);
    // Activating the skip link moves focus to <main>.
    await page.keyboard.press("Shift+Tab");
    await page.keyboard.press("Enter");
    await expect(page.locator("#main-content")).toBeFocused();
  });

  test("mobile 390px: drawer opens/closes, navigates, no horizontal scroll", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/dashboard");
    await expect(page.locator("#app-sidebar")).toBeHidden();
    const noHScroll = () => page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth);
    expect(await noHScroll()).toBe(true);

    const open = page.getByRole("button", { name: cat.id.layout.openMenu });
    await open.click();
    const drawer = page.getByTestId("nav-drawer");
    await expect(drawer).toBeVisible();
    await expectAccessible(page, "mobile drawer");
    await page.screenshot({ path: `${artifactsDir}/shell-mobile-drawer.png` });
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();

    await open.click();
    await drawer.locator('a[data-menu="maintenance"]').click();
    await expect(drawer).toBeHidden();
    await expect(page).toHaveURL(/\/maintenance$/);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(cat.id.sidebar.maintenance);
    expect(await noHScroll()).toBe(true);
    await page.goto("/rak/tidak/ada");
    expect(await noHScroll()).toBe(true);
    await page.screenshot({ path: `${artifactsDir}/shell-mobile-404.png`, fullPage: true });
    expect(problems).toEqual([]);
  });

  for (const theme of ["dark", "light"] as const) {
    for (const lng of ["id", "en"] as const) {
      test(`${theme} / ${lng}: axe clean on dashboard and a coming-soon page`, async ({ page }) => {
        const problems = await watchProblems(page);
        await page.addInitScript(
          ([t, l]) => {
            window.localStorage.setItem("sentra.theme", t);
            window.localStorage.setItem("sentra.locale", l);
          },
          [theme, lng],
        );
        await page.goto("/dashboard");
        await expect(page.getByRole("heading", { level: 1 })).toHaveText(cat[lng].sidebar.dashboard);
        await expect(page.getByTestId("dashboard-empty")).toContainText(cat[lng].dashboard.emptyTitle);
        await expectAccessible(page, `${theme}/${lng} dashboard`);
        await page.screenshot({ path: `${artifactsDir}/shell-${theme}-${lng}-dashboard.png` });
        await page.goto("/alarms");
        await expect(page.getByTestId("coming-soon")).toContainText(cat[lng].pages.comingSoon.title);
        await expectAccessible(page, `${theme}/${lng} coming soon`);
        expect(problems).toEqual([]);
      });
    }
  }
});
