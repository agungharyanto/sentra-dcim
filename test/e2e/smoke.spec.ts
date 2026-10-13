import { expect, test } from "@playwright/test";

import { catalog, pass, user, watchProblems } from "./helpers";

const id = { shell: catalog("id", "shell"), common: catalog("id", "common") };
const en = { shell: catalog("en", "shell"), common: catalog("en", "common") };

test.describe("web UI smoke", () => {
  test.use({ httpCredentials: { username: user, password: pass } });

  test("loads behind basic auth and switches ID ↔ EN without console errors or CSP violations", async ({ page }) => {
    test.skip(!user || !pass, "SENTRA_WEB_USER / SENTRA_WEB_PASS not set");
    const problems = await watchProblems(page);

    const res = await page.goto("/");
    expect(res?.status()).toBe(200);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(id.shell.placeholder.title);
    await expect(page.locator("html")).toHaveAttribute("lang", "id");
    await expect(page.getByText(id.shell.sidebar.dashboard)).toBeVisible();
    await expect(page.getByTestId("timezone")).not.toBeEmpty();
    await expect(page.getByTestId("version")).toHaveText(/^v\d+\.\d+\.\d+/);

    await page.getByRole("button", { name: new RegExp(en.common.language.en) }).click();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(en.shell.placeholder.title);
    await expect(page.locator("html")).toHaveAttribute("lang", "en");
    await expect(page.getByText(en.shell.sidebar.dashboard)).toBeVisible();

    // The choice survives a reload (stored in this browser).
    await page.reload();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(en.shell.placeholder.title);

    await page.getByRole("button", { name: new RegExp(id.common.language.id) }).click();
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(id.shell.placeholder.title);

    // Deep link is served by the SPA fallback.
    const deep = await page.goto("/apa-saja/deep/link");
    expect(deep?.status()).toBe(200);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(id.shell.placeholder.title);

    expect(problems).toEqual([]);
  });
});

test("rejects the UI without credentials (401)", async ({ browser, baseURL }) => {
  const ctx = await browser.newContext({ ignoreHTTPSErrors: true });
  const res = await ctx.request.get(`${baseURL}/`);
  expect(res.status()).toBe(401);
  await ctx.close();
});
