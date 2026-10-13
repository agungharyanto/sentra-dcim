import AxeBuilder from "@axe-core/playwright";
import { expect, type Page, test } from "@playwright/test";

import { artifactsDir, catalog, pass, stateFile, user, watchProblems } from "./helpers";

const cat = {
  id: { help: catalog("id", "help"), shell: catalog("id", "shell"), ds: catalog("id", "ds") },
  en: { help: catalog("en", "help"), shell: catalog("en", "shell"), ds: catalog("en", "ds") },
};

async function expectAccessible(page: Page, label: string) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  const bad = results.violations
    .filter((v) => v.impact === "serious" || v.impact === "critical")
    .map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`);
  expect(bad, `axe violations (${label})`).toEqual([]);
}

const article = (page: Page) => page.locator("article[data-slug]");
const helpNav = (page: Page, lng: "id" | "en" = "id") => page.getByRole("navigation", { name: cat[lng].help.pagesNav });

test.describe("Help Center", () => {
  test.skip(!user || !pass, "SENTRA_WEB_USER / SENTRA_WEB_PASS not set");
  test.use({ httpCredentials: { username: user, password: pass } });

  test("opens from the sidebar, navigates pages and links, searches", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/dashboard");
    await page.getByRole("navigation", { name: cat.id.shell.layout.mainNav }).locator('a[data-menu="help"]').click();
    await expect(page).toHaveURL(/\/help$/);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(cat.id.shell.sidebar.help);
    await expect(article(page)).toHaveAttribute("data-slug", "help");

    await helpNav(page).locator('a[data-help="getting-started"]').click();
    await expect(page).toHaveURL(/\/help\/getting-started$/);
    await expect(article(page)).toHaveAttribute("data-slug", "getting-started");
    // Internal Markdown link (rewritten by the backend) navigates within the app.
    await article(page).getByRole("link").filter({ hasText: /Dasbor/ }).first().click();
    await expect(page).toHaveURL(/\/help\/dashboard$/);
    await expect(article(page)).toHaveAttribute("data-slug", "dashboard");

    await page.getByTestId("help-search").fill("keyboard");
    const results = page.getByTestId("help-results");
    await expect(results.getByRole("link")).toHaveCount(1);
    await results.getByRole("link").click();
    await expect(article(page)).toHaveAttribute("data-slug", "getting-started");
    expect(problems).toEqual([]);
  });

  test('"?" in the header opens the help of the active menu', async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByTestId("help-button")).toHaveAttribute("href", "/help/dashboard");
    await page.getByTestId("help-button").click();
    await expect(article(page)).toHaveAttribute("data-slug", "dashboard");
    await page.goto("/alarms");
    await expect(page.getByTestId("help-button")).toHaveAttribute("href", "/help");
  });

  test("API reference is read-only and complete", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/help");
    await helpNav(page).locator('a[data-help="api-reference"]').click();
    await expect(page).toHaveURL(/\/help\/api\/reference$/);
    const ref = page.getByTestId("api-reference");
    await expect(ref).toContainText("Sentra DCIM API");
    const op = ref.locator('[data-operation="GET /api/v1/help/pages/{slug}"]');
    await op.getByTestId("operation-toggle").click();
    await expect(op.getByTestId("operation-toggle")).toHaveAttribute("aria-expanded", "true");
    await expect(op).toContainText("locale");
    await expect(op.locator('[data-status-code="404"]')).toBeVisible();
    // Access labels from x-sentra-access.
    await expect(ref.locator('[data-operation="GET /api/v1/health"] [data-access]')).toHaveText(cat.id.help.api.access.public);
    await expect(ref.locator('[data-operation="GET /api/v1/ready"] [data-access]')).toHaveText(cat.id.help.api.access.internal);
    await expect(ref.locator('[data-operation="GET /api/v1/ui/bootstrap"] [data-access]')).toHaveText(cat.id.help.api.access.login);
    // Read-only: only disclosure toggles and copy buttons, no forms.
    const toggles = await ref.getByTestId("operation-toggle").count();
    const copies = await ref.getByRole("button", { name: /^Salin path \// }).count();
    await expect(ref.getByRole("button")).toHaveCount(toggles + copies);
    expect(copies).toBe(toggles);
    await expect(ref.locator("form")).toHaveCount(0);
    await expectAccessible(page, "api reference");
    await page.screenshot({ path: `${artifactsDir}/help-api-reference.png`, fullPage: true });
    expect(problems).toEqual([]);
  });

  test("copy path button: copies the plain path, never toggles the endpoint", async ({ page, context, baseURL }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin: baseURL ?? "" });
    const problems = await watchProblems(page);
    await page.goto("/help/api/reference");
    const row = page.locator('[data-operation="GET /api/v1/health"]');
    const copy = row.getByRole("button", { name: "Salin path /api/v1/health" });

    // Hidden until the row is hovered or the button is focused.
    await expect(copy).toHaveCSS("opacity", "0");
    await row.hover();
    await expect(copy).toHaveCSS("opacity", "1");
    await copy.click();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe("/api/v1/health");
    await expect(copy).toHaveAttribute("data-copied", "true");
    await expect(row.getByTestId("operation-toggle")).toHaveAttribute("aria-expanded", "false");
    await expect(row).toHaveAttribute("data-open", "false");
    await expect(copy.locator('[aria-live="polite"]')).toHaveText(cat.id.ds.copy.copied);
    await page.screenshot({ path: `${artifactsDir}/help-api-copy-hover-dark.png`, clip: { x: 0, y: 0, width: 1280, height: 420 } });
    await expect(copy).toHaveAttribute("data-copied", "false", { timeout: 4000 });

    // Keyboard: Tab from the endpoint toggle lands on its copy button, which becomes visible (:focus-visible).
    await page.mouse.move(0, 0);
    const ready = page.locator('[data-operation="GET /api/v1/ready"]');
    await ready.getByTestId("operation-toggle").focus();
    await page.keyboard.press("Tab");
    const copyReady = ready.getByRole("button", { name: "Salin path /api/v1/ready" });
    await expect(copyReady).toBeFocused();
    await expect(copyReady).toHaveCSS("opacity", "1");
    expect(problems).toEqual([]);
  });

  test("copy button in light theme and on touch screens", async ({ page, browser, baseURL }) => {
    await page.addInitScript(() => window.localStorage.setItem("sentra.theme", "light"));
    await page.goto("/help/api/reference");
    const row = page.locator('[data-operation="GET /api/v1/health"]');
    await row.hover();
    await expect(row.getByRole("button", { name: "Salin path /api/v1/health" })).toHaveCSS("opacity", "1");
    await page.screenshot({ path: `${artifactsDir}/help-api-copy-hover-light.png`, clip: { x: 0, y: 0, width: 1280, height: 420 } });

    // Touch device (hover: none): always visible, no horizontal scroll.
    const touch = await browser.newContext({
      viewport: { width: 390, height: 844 },
      deviceScaleFactor: 2, // legible icons in the screenshot used for the visual check
      hasTouch: true,
      isMobile: true,
      ignoreHTTPSErrors: true,
      httpCredentials: { username: user, password: pass },
      storageState: stateFile,
      reducedMotion: "reduce",
    });
    const mobile = await touch.newPage();
    await mobile.goto(`${baseURL}/help/api/reference`);
    const mcopy = mobile.locator('[data-operation="GET /api/v1/health"]').getByRole("button", { name: "Salin path /api/v1/health" });
    await expect(mcopy).toHaveCSS("opacity", "1");
    expect(await mobile.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
    await mobile.screenshot({ path: `${artifactsDir}/help-api-mobile.png`, clip: { x: 0, y: 560, width: 390, height: 420 } });
    await touch.close();
  });

  test("switching ID ↔ EN reloads the same page in the other language", async ({ page }) => {
    await page.goto("/help/dashboard");
    await expect(article(page)).toHaveAttribute("lang", "id");
    await expect(article(page).getByRole("heading").first()).toHaveText("Dasbor");
    await page.getByRole("button", { name: /English/ }).click();
    await expect(article(page)).toHaveAttribute("lang", "en");
    await expect(article(page).getByRole("heading").first()).toHaveText("Dashboard");
    await expect(helpNav(page, "en").locator('a[data-help="getting-started"]')).toHaveText("Getting started");
  });

  for (const theme of ["dark", "light"] as const) {
    for (const lng of ["id", "en"] as const) {
      test(`${theme} / ${lng}: axe clean, no console/CSP errors`, async ({ page }) => {
        const problems = await watchProblems(page);
        await page.addInitScript(
          ([t, l]) => {
            window.localStorage.setItem("sentra.theme", t);
            window.localStorage.setItem("sentra.locale", l);
          },
          [theme, lng],
        );
        await page.goto("/help/getting-started");
        await expect(article(page)).toHaveAttribute("lang", lng);
        await expectAccessible(page, `${theme}/${lng} getting-started`);
        await page.screenshot({ path: `${artifactsDir}/help-${theme}-${lng}.png`, fullPage: true });
        expect(problems).toEqual([]);
      });
    }
  }

  test("mobile 390px: list above content, no horizontal scroll", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.setViewportSize({ width: 390, height: 844 });
    const noHScroll = () => page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth);
    for (const path of ["/help", "/help/getting-started", "/help/api", "/help/api/reference"]) {
      await page.goto(path);
      await expect(page.getByTestId("help-center")).toBeVisible();
      await page.waitForLoadState("networkidle");
      expect(await noHScroll(), path).toBe(true);
    }
    await page.goto("/help/getting-started");
    await expect(article(page)).toBeVisible();
    await page.screenshot({ path: `${artifactsDir}/help-mobile.png`, fullPage: true });
    expect(problems).toEqual([]);
  });
});
