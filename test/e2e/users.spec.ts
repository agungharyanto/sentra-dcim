import AxeBuilder from "@axe-core/playwright";
import { expect, type Page, test } from "@playwright/test";

import { artifactsDir, catalog, watchProblems } from "./helpers";

const tx = catalog("id", "users");
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

async function signedInUser(page: Page) {
  return (await (await page.request.get("/api/v1/auth/me")).json()).user as { id: string; username: string; permissions: string[] };
}

test.describe("users & roles", () => {

  test("list, roles tab and own account (read-only), desktop + mobile, axe", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/dashboard");
    const me = await signedInUser(page);
    test.skip(!me.permissions.includes("users.read"), "test account has no users.read");

    await page.goto("/users");
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(shell.sidebar.users);
    await expect(page.locator(`[data-testid="user-row"][data-username="${me.username}"]`)).toContainText(tx.list.you);
    await expectAccessible(page, "users list");
    await page.screenshot({ path: `${artifactsDir}/users-list.png`, fullPage: true });

    await page.getByRole("tab", { name: tx.tabs.roles }).click();
    await expect(page).toHaveURL(/\/users\?tab=roles$/);
    await expect(page.getByTestId("roles-table").locator('[data-role="admin"]').first()).toBeVisible();
    await expectAccessible(page, "roles");
    await page.screenshot({ path: `${artifactsDir}/users-roles.png`, fullPage: true });

    await page.goto(`/users/${me.id}`);
    await expect(page.getByText(tx.detail.self)).toBeVisible();
    await expect(page.getByTestId("toggle-status")).toHaveCount(0);
    await expectAccessible(page, "own user detail");
    await page.screenshot({ path: `${artifactsDir}/users-detail-self.png`, fullPage: true });

    await page.setViewportSize(mobile);
    expect(await noHorizontalScroll(page)).toBe(true);
    await page.screenshot({ path: `${artifactsDir}/users-detail-self-mobile.png`, fullPage: true });
    await page.goto("/users");
    await expect(page.getByTestId("user-card").first()).toBeVisible();
    expect(await noHorizontalScroll(page)).toBe(true);
    await expectAccessible(page, "users list mobile");
    await page.screenshot({ path: `${artifactsDir}/users-list-mobile.png`, fullPage: true });
    expect(problems).toEqual([]);
  });

  test("without permission: no menu, Access denied page (permissions mocked)", async ({ page }) => {
    const problems = await watchProblems(page);
    // Pretend the account is a Viewer; nothing is changed on the server.
    await page.route("**/api/v1/auth/me", async (route) => {
      const res = await route.fetch();
      const body = await res.json();
      body.user.roles = ["viewer"];
      body.user.permissions = [];
      await route.fulfill({ response: res, json: body });
    });
    await page.goto("/users");
    await expect(page.getByTestId("forbidden")).toContainText(shell.pages.forbidden.title);
    await expect(page.locator('a[data-menu="users"]')).toHaveCount(0);
    await expectAccessible(page, "forbidden");
    await page.screenshot({ path: `${artifactsDir}/users-forbidden.png`, fullPage: true });
    expect(problems).toEqual([]);
  });

  // Creates and changes accounts: only on the CI throwaway stack (E2E_MUTATIONS=1), never against production.
  test("create with temporary password, disable/enable, reset (CI only)", async ({ page, browser }) => {
    test.skip(process.env.E2E_MUTATIONS !== "1", "mutating test runs only on the CI throwaway stack");
    const problems = await watchProblems(page);
    const username = `e2e.${Date.now().toString(36)}`;
    await page.goto("/users");
    await page.getByTestId("add-user").click();
    const dialog = page.getByTestId("create-user-dialog");
    await dialog.getByLabel(tx.create.username).fill(username);
    await dialog.getByLabel(tx.create.displayName).fill("Teknisi E2E");
    await dialog.getByRole("checkbox", { name: new RegExp(tx.roles.technician) }).check();
    await expectAccessible(page, "create user dialog");
    await page.screenshot({ path: `${artifactsDir}/users-create.png` });
    await dialog.getByRole("button", { name: tx.create.submit }).click();
    const value = dialog.getByTestId("temporary-password-value");
    await expect(value).toHaveText(/^[A-Za-z2-9]{4}(-[A-Za-z2-9]{4}){3}$/);
    const temp = (await value.textContent()) ?? "";
    await expectAccessible(page, "temporary password");
    // Even a throwaway password is masked in the uploaded screenshot.
    await page.screenshot({ path: `${artifactsDir}/users-temp-password.png`, mask: [value] });
    await dialog.getByRole("button", { name: tx.temp.done }).click();
    await expect(dialog).toHaveCount(0);

    // The new account signs in with the temporary password and must change it.
    const other = await browser.newContext({ storageState: { cookies: [], origins: [] } });
    const login = await other.request.post("/api/v1/auth/login", {
      headers: { "X-Sentra-CSRF": "login", Origin: new URL(page.url()).origin },
      data: { username, password: temp },
    });
    expect(login.status()).toBe(200);
    expect((await login.json()).user.must_change_password).toBe(true);

    await page.getByTestId("users-search").fill(username);
    const row = page.locator(`[data-testid="user-row"][data-username="${username}"]`);
    await row.getByRole("link").click();
    await expect(page.getByTestId("user-detail")).toContainText("Teknisi E2E");
    await page.screenshot({ path: `${artifactsDir}/users-detail.png`, fullPage: true });

    await page.getByTestId("toggle-status").click();
    await expect(page.getByTestId("confirm-dialog")).toContainText(tx.detail.disableBody);
    await page.screenshot({ path: `${artifactsDir}/users-disable-confirm.png` });
    await page.getByTestId("confirm").click();
    await expect(page.getByTestId("toggle-status")).toHaveText(tx.detail.enable);
    // Disabling revoked the other browser's session immediately.
    expect((await other.request.get("/api/v1/auth/me")).status()).toBe(401);
    await page.getByTestId("toggle-status").click();
    await page.getByTestId("confirm").click();
    await expect(page.getByTestId("toggle-status")).toHaveText(tx.detail.disable);

    await page.getByTestId("reset-password").click();
    await page.getByTestId("confirm").click();
    await expect(page.getByTestId("temporary-password-value")).toHaveText(/^[A-Za-z2-9]{4}(-[A-Za-z2-9]{4}){3}$/);
    await page.getByRole("button", { name: tx.temp.done }).click();
    await other.close();
    expect(problems).toEqual([]);
  });
});
