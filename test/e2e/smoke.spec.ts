import { expect, test } from "@playwright/test";

import { catalog, pass, user, watchProblems } from "./helpers";

const id = { shell: catalog("id", "shell"), common: catalog("id", "common") };
const en = { shell: catalog("en", "shell"), common: catalog("en", "common") };

test.describe("web UI smoke", () => {
  test.use({ httpCredentials: { username: user, password: pass } });

  test("loads behind basic auth, opens the dashboard and switches ID ↔ EN without console/CSP errors", async ({ page }) => {
    test.skip(!user || !pass, "SENTRA_WEB_USER / SENTRA_WEB_PASS not set");
    const problems = await watchProblems(page);

    const res = await page.goto("/");
    expect(res?.status()).toBe(200);
    await expect(page).toHaveURL(/\/dashboard$/);
    const h1 = page.getByRole("heading", { level: 1 });
    await expect(h1).toHaveText(id.shell.sidebar.dashboard);
    await expect(page.getByTestId("dashboard-empty")).toContainText(id.shell.dashboard.emptyTitle);
    await expect(page.locator("html")).toHaveAttribute("lang", "id");
    await expect(page.getByTestId("app-version")).toContainText(/v\d+\.\d+\.\d+/);
    await expect(page.getByTestId("clock")).not.toBeEmpty();

    await page.getByRole("button", { name: new RegExp(en.common.language.en) }).click();
    await expect(h1).toHaveText(en.shell.sidebar.dashboard);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");

    // The choice survives a reload (stored in this browser).
    await page.reload();
    await expect(h1).toHaveText(en.shell.sidebar.dashboard);
    await page.getByRole("button", { name: new RegExp(id.common.language.id) }).click();
    await expect(h1).toHaveText(id.shell.sidebar.dashboard);

    // Unknown deep link: served by the SPA fallback, rendered as 404 inside the shell.
    const deep = await page.goto("/apa-saja/deep/link");
    expect(deep?.status()).toBe(200);
    await expect(h1).toHaveText(id.shell.pages.notFound.title);
    await expect(page.getByTestId("not-found")).toContainText("/apa-saja/deep/link");

    expect(problems).toEqual([]);
  });
});

test("rejects the UI without credentials (401)", async ({ browser, baseURL }) => {
  const ctx = await browser.newContext({ ignoreHTTPSErrors: true });
  const res = await ctx.request.get(`${baseURL}/`);
  expect(res.status()).toBe(401);
  await ctx.close();
});
