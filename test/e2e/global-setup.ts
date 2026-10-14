import { request } from "@playwright/test";

import { appPass, appTotpCode, appTotpSecret, appUser, freshTotp, pass, stateFile, user } from "./helpers";

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
  const body = (await res.json()) as { user: { must_change_password: boolean; mfa_enabled: boolean } };
  if (!body.user.mfa_enabled) {
    await ctx.dispose();
    throw new Error(`account ${appUser} has no MFA yet (sign in once in the browser and set up the authenticator)`);
  }
  // MFA step (v0.3.3): CSRF token from the cookie just set by the login.
  const csrf = (await ctx.storageState()).cookies.find((c) => c.name === "__Host-sentra_csrf")?.value ?? "";
  const code = appTotpSecret ? await freshTotp(appTotpSecret) : appTotpCode;
  if (!code) {
    await ctx.dispose();
    throw new Error("no MFA code: SENTRA_APP_TOTP (run.sh asks for it) or SENTRA_APP_TOTP_SECRET (CI)");
  }
  const mfa = await ctx.post("/api/v1/auth/mfa/verify", {
    data: { code },
    headers: { Origin: baseURL, "X-Sentra-CSRF": csrf },
  });
  if (mfa.status() !== 204) {
    await ctx.dispose();
    throw new Error(`MFA verification failed: HTTP ${mfa.status()} — no further attempts (account lockout after 5)`);
  }
  if (body.user.must_change_password) {
    await ctx.dispose();
    throw new Error(`account ${appUser} must change its password first (sign in once in the browser)`);
  }
  await ctx.storageState({ path: stateFile });
  await ctx.dispose();
}
