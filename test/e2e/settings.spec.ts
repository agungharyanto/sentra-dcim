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

  // Read-only: the page and "your IP", desktop + mobile (production-safe; nothing is saved).
  test("Allowed IP page: your IP, mode, list, always-public paths, desktop + mobile, axe", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/dashboard");
    test.skip(!(await me(page)).user.permissions.includes("settings.manage"), "test account has no settings.manage");
    await page.goto("/settings/allowed-ip");
    await expect(page.getByTestId("allowlist-your-ip")).toContainText(/\d|:/);
    await expect(page.getByTestId("allowlist-always-public")).toContainText("/.well-known/acme-challenge/*");
    await expectAccessible(page, "allowed ip");
    await page.screenshot({ path: `${artifactsDir}/settings-allowlist.png`, fullPage: true });
    await page.setViewportSize(mobile);
    expect(await noHorizontalScroll(page)).toBe(true);
    await page.screenshot({ path: `${artifactsDir}/settings-allowlist-mobile.png`, fullPage: true });
    expect(problems).toEqual([]);
  });

  // Changes the allowlist and switches to enforce: CI throwaway stack only. Always ends in mode off.
  test("Allowed IP: add own IP, enforce with confirmation, keep, back to off (CI only)", async ({ page }) => {
    test.skip(process.env.E2E_MUTATIONS !== "1", "changes the allowlist: runs only on the CI throwaway stack");
    const problems = await watchProblems(page);
    const al = catalog("id", "settings").allowlist;
    await page.goto("/settings/allowed-ip");
    try {
      await page.getByTestId("allowlist-mode-enforce").check();
      await expect(page.getByTestId("allowlist-guard")).toBeVisible();
      await expect(page.getByTestId("allowlist-mode-save")).toBeDisabled();
      await page.getByTestId("allowlist-add-mine").click();
      await expect(page.getByTestId("allowlist-your-ip")).toHaveAttribute("data-covered", "true");
      await page.getByTestId("allowlist-cidr").fill("192.0.2.0/24");
      await page.getByTestId("allowlist-add-submit").click();
      await expect(page.getByTestId("allowlist-row")).toHaveCount(2);
      await page.getByTestId("allowlist-mode-enforce").check();
      await page.getByTestId("allowlist-mode-save").click();
      await page.getByTestId("confirm-dialog").getByTestId("confirm").click();
      await expect(page.getByTestId("allowlist-pending")).toBeVisible();
      await page.screenshot({ path: `${artifactsDir}/settings-allowlist-pending.png`, fullPage: true });
      await page.getByTestId("allowlist-keep").click();
      await expect(page.getByTestId("allowlist-pending")).toHaveCount(0);
      await expect(page.getByText(al.kept)).toBeVisible();
      // Still inside: the panel keeps working under enforce.
      await page.goto("/dashboard");
      await expect(page.getByTestId("dashboard-empty")).toBeVisible();
    } finally {
      await page.goto("/settings/allowed-ip");
      await page.getByTestId("allowlist-mode-off").check();
      await page.getByTestId("allowlist-mode-save").click();
      await expect(page.locator('[data-testid="allowlist-mode-off"]')).toBeChecked();
    }
    expect(problems).toEqual([]);
  });

  // Read-only: the session & password policy page, desktop + mobile.
  test("Session & password policy page, desktop + mobile, axe", async ({ page }) => {
    const problems = await watchProblems(page);
    await page.goto("/dashboard");
    test.skip(!(await me(page)).user.permissions.includes("settings.manage"), "test account has no settings.manage");
    await page.goto("/settings/session-password");
    await expect(page.getByTestId("policy-idle_minutes")).toHaveValue(/\d+/);
    await expect(page.getByTestId("policy-expiry_days")).toHaveValue(/\d+/);
    await expectAccessible(page, "policy settings");
    await page.screenshot({ path: `${artifactsDir}/settings-policy.png`, fullPage: true });
    await page.setViewportSize(mobile);
    expect(await noHorizontalScroll(page)).toBe(true);
    await page.screenshot({ path: `${artifactsDir}/settings-policy-mobile.png`, fullPage: true });
    expect(problems).toEqual([]);
  });

  // The idle warning, with the activity response mocked to a near deadline (nothing changes on the server).
  test("idle warning 2 minutes before sign-out (mocked deadline), axe", async ({ page }) => {
    const problems = await watchProblems(page);
    let near = true;
    await page.route("**/api/v1/auth/activity", async (route) => {
      const res = await route.fetch();
      const body = await res.json();
      if (near) body.session.idle_expires_at = new Date(Date.now() + 90_000).toISOString();
      await route.fulfill({ response: res, json: body });
    });
    await page.goto("/dashboard");
    const dialog = page.getByTestId("session-warning");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByTestId("session-countdown")).toHaveText(/^1:[0-3]\d$/);
    await expectAccessible(page, "session warning");
    await page.screenshot({ path: `${artifactsDir}/session-warning.png` });
    near = false;
    await dialog.getByTestId("session-stay").click();
    await expect(dialog).toHaveCount(0);
    expect(problems).toEqual([]);
  });

  // Creates a Viewer account: CI throwaway stack only.
  test("kiosk card: refused while Allowed IP is off (CI only)", async ({ page }) => {
    test.skip(process.env.E2E_MUTATIONS !== "1", "creates an account: runs only on the CI throwaway stack");
    const problems = await watchProblems(page);
    await page.goto("/dashboard");
    const csrf = (await page.context().cookies()).find((c) => c.name === "__Host-sentra_csrf")?.value ?? "";
    const res = await page.request.post("/api/v1/users", {
      headers: { "X-Sentra-CSRF": csrf, Origin: new URL(page.url()).origin },
      data: { username: `wall.${Date.now().toString(36)}`, display_name: "Layar NOC", roles: ["viewer"] },
    });
    expect(res.status()).toBe(201);
    const id = (await res.json()).user.id as string;
    await page.goto(`/users/${id}`);
    const card = page.getByTestId("kiosk-card");
    await expect(card).toBeVisible();
    await card.getByTestId("kiosk-toggle").click();
    await page.getByTestId("confirm-dialog").getByTestId("confirm").click();
    await expect(page.getByText(catalog("id", "errors").code.kiosk_requires_allowlist)).toBeVisible();
    await page.screenshot({ path: `${artifactsDir}/users-kiosk-refused.png`, fullPage: true });
    expect(problems).toEqual([]);
  });
});
