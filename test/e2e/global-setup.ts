import { request } from "@playwright/test";

import { appPass, appUser, pass, stateFile, user } from "./helpers";

/**
 * Signs in ONCE through the API (basic_auth + session login with Origin and CSRF header) and stores the cookies
 * for all specs. One login per run keeps far below the login rate limit and the account lockout; a wrong password
 * stops the run here instead of in every spec.
 */
export default async function globalSetup() {
  const baseURL = process.env.BASE_URL ?? "";
  if (!appUser || !appPass) throw new Error("SENTRA_APP_USER / SENTRA_APP_PASS not set (see test/e2e/README.md)");
  const ctx = await request.newContext({
    baseURL,
    ignoreHTTPSErrors: baseURL.startsWith("https://localhost"),
    httpCredentials: { username: user, password: pass },
  });
  const res = await ctx.post("/api/v1/auth/login", {
    data: { username: appUser, password: appPass },
    headers: { Origin: baseURL, "X-Sentra-CSRF": "login" },
  });
  if (res.status() !== 200) {
    await ctx.dispose();
    throw new Error(`app login failed: HTTP ${res.status()} — no further attempts (account lockout after 5)`);
  }
  const body = (await res.json()) as { user: { must_change_password: boolean } };
  if (body.user.must_change_password) {
    await ctx.dispose();
    throw new Error(`account ${appUser} must change its password first (sign in once in the browser)`);
  }
  await ctx.storageState({ path: stateFile });
  await ctx.dispose();
}
