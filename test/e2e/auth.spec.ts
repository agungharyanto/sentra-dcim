import AxeBuilder from "@axe-core/playwright";
import { expect, type Page, test } from "@playwright/test";

import { appPass, appTotpSecret, appUser, artifactsDir, catalog, freshTotp, watchProblems } from "./helpers";

const id = catalog("id", "auth");
const mobile = { width: 390, height: 844 };
const signedOut = { cookies: [], origins: [] };

async function expectAccessible(page: Page, label: string) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  const bad = results.violations
    .filter((v) => v.impact === "serious" || v.impact === "critical")
    .map((v) => `${v.id} (${v.impact}): ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`);
  expect(bad, `axe violations (${label})`).toEqual([]);
}

/** The signed-out probe of /auth/me is an expected 401; anything else is a problem. */
const unexpected = (problems: string[]) => problems.filter((p) => !p.includes("status of 401"));

const noHorizontalScroll = (page: Page) =>
  page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth);

test.describe("auth (signed out)", () => {
  test.use({ storageState: signedOut });

  test("login page and generic login error (mocked, no real attempt), desktop + mobile, axe", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/help");
    await expect(page).toHaveURL(/\/login\?next=%2Fhelp$/);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(id.login.title);
    await expectAccessible(page, "login");
    await page.screenshot({ path: `${artifactsDir}/auth-login.png`, fullPage: true });

    // The error state is mocked: a real wrong password would count towards the lockout of the test account.
    await page.route("**/api/v1/auth/login", (route) =>
      route.fulfill({ status: 401, json: { error: { code: "invalid_credentials", message: "invalid username or password" } } }),
    );
    await page.getByLabel(id.login.username).fill("operator.contoh");
    await page.getByLabel(id.login.password).fill("bukan-kata-sandi");
    await page.getByRole("button", { name: id.login.submit }).click();
    await expect(page.getByTestId("login-error")).toHaveText(id.login.invalid);
    await expect(page.getByLabel(id.login.password)).toHaveValue("");
    await expectAccessible(page, "login error");
    await page.screenshot({ path: `${artifactsDir}/auth-login-error.png`, fullPage: true });

    await page.setViewportSize(mobile);
    expect(await noHorizontalScroll(page)).toBe(true);
    await page.screenshot({ path: `${artifactsDir}/auth-login-error-mobile.png`, fullPage: true });
    expect(unexpected(problems)).toEqual([]);
  });

  test("real login through the form with MFA, user name in the shell, logout", async ({ page }) => {
    // A second real sign-in needs a second code: only with the CI throwaway secret (never a production phone).
    test.skip(!appUser || !appPass || !appTotpSecret, "needs SENTRA_APP_TOTP_SECRET (CI throwaway account)");
    const problems = await watchProblems(page);
    await page.goto("/login");
    await page.getByLabel(id.login.username).fill(appUser);
    await page.getByLabel(id.login.password).fill(appPass);
    await page.getByRole("button", { name: id.login.submit }).click();
    await expect(page).toHaveURL(/\/mfa\?next=%2Fdashboard$/);
    test.setTimeout(90_000); // waits for a fresh 30 s step
    await page.getByLabel(id.mfa.codeLabel).fill(await freshTotp(appTotpSecret));
    await page.getByRole("button", { name: id.mfa.verify }).click();
    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(page.getByTestId("user-name")).not.toBeEmpty();

    // HttpOnly session cookie: invisible to page scripts; the CSRF cookie is readable (double-submit).
    expect(await page.evaluate(() => document.cookie)).not.toContain("__Host-sentra_session");
    expect(await page.evaluate(() => document.cookie)).toContain("__Host-sentra_csrf=");

    await page.getByTestId("user-menu").click();
    await page.getByTestId("logout").click();
    await expect(page).toHaveURL(/\/login/);
    const me = await page.request.get("/api/v1/auth/me");
    expect(me.status()).toBe(401);
    expect(unexpected(problems)).toEqual([]);
  });
});

test.describe("auth (signed in)", () => {

  test("forced password change page (must_change mocked; never submitted), desktop + mobile, axe", async ({ page }) => {
    const problems = await watchProblems(page);
    // Pretend the account still has its temporary password; the real account is not changed.
    await page.route("**/api/v1/auth/me", async (route) => {
      const res = await route.fetch();
      const body = await res.json();
      body.user.must_change_password = true;
      await route.fulfill({ response: res, json: body });
    });
    await page.route("**/api/v1/auth/password", (route) => route.abort());
    await page.goto("/dashboard");
    await expect(page).toHaveURL(/\/change-password\?next=%2Fdashboard$/);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(id.forced.title);
    await expectAccessible(page, "forced password change");
    await page.screenshot({ path: `${artifactsDir}/auth-forced-password.png`, fullPage: true });

    // Local validation only (no request): mismatching confirmation.
    await page.getByLabel(id.password.current).fill("sementara-1234567");
    await page.getByLabel(id.password.new, { exact: true }).fill("kalimat panjang yang baru");
    await page.getByLabel(id.password.confirm).fill("kalimat panjang yang lain");
    await page.getByRole("button", { name: id.password.submit }).click();
    await expect(page.getByTestId("password-error")).toHaveText(id.password.mismatch);
    await page.setViewportSize(mobile);
    expect(await noHorizontalScroll(page)).toBe(true);
    await page.screenshot({ path: `${artifactsDir}/auth-forced-password-mobile.png`, fullPage: true });
    expect(problems).toEqual([]);
  });

  test("shell shows the user; Sesi saya lists the current session, desktop + mobile, axe", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/dashboard");
    const me = await (await page.request.get("/api/v1/auth/me")).json();
    await expect(page.getByTestId("user-name")).toHaveText(me.user.display_name);
    await page.screenshot({ path: `${artifactsDir}/auth-shell-signed-in.png` });

    await page.getByTestId("user-menu").click();
    await expect(page.getByRole("menuitem", { name: id.user.logout })).toBeVisible();
    await page.screenshot({ path: `${artifactsDir}/auth-user-menu.png`, clip: { x: 640, y: 0, width: 640, height: 320 } });
    await page.getByRole("menuitem", { name: id.sessions.title }).click();
    await expect(page).toHaveURL(/\/account\/sessions$/);
    await expect(page.getByRole("heading", { level: 1 })).toHaveText(id.sessions.title);
    const rows = page.getByTestId("session-row");
    await expect(rows.first()).toBeVisible();
    await expect(page.locator('[data-testid="session-row"][data-current="true"]')).toHaveCount(1);
    await expectAccessible(page, "sessions");
    await page.screenshot({ path: `${artifactsDir}/auth-sessions.png`, fullPage: true });

    await page.setViewportSize(mobile);
    await expect(page.getByTestId("session-card").first()).toBeVisible();
    expect(await noHorizontalScroll(page)).toBe(true);
    await expectAccessible(page, "sessions mobile");
    await page.screenshot({ path: `${artifactsDir}/auth-sessions-mobile.png`, fullPage: true });
    expect(problems).toEqual([]);
  });
});
