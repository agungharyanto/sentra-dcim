import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { testBootstrap, testMe } from "@/test/bootstrap";

const tx = resources.id.auth;
const shell = resources.id.shell;
const CODES = ["abcde-fghjk", "bcdef-ghjkm", "cdefg-hjkmn", "defgh-jkmnp", "efghj-kmnpq", "fghjk-mnpqr", "ghjkm-npqrs", "hjkmn-pqrst", "jkmnp-qrstu", "kmnpq-rstuv"];

const json = (b: unknown, status = 200) => new Response(status === 204 ? null : JSON.stringify(b), { status });
const err = (code: string, status: number) => json({ error: { code, message: "x" } }, status);

/** Fake API with an MFA state machine: pending until verify/confirm succeeds with "123456" (or a recovery code). */
function fakeApi(opts: { enabled: boolean; mustChange?: boolean }) {
  const state = { verified: false, enabled: opts.enabled, mustChange: opts.mustChange ?? false, remaining: 10 };
  const calls: { method: string; url: string; body: unknown }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      const body = init?.body ? (JSON.parse(String(init.body)) as { code?: string }) : undefined;
      calls.push({ method, url, body });
      const me = () => testMe({ mfa_enabled: state.enabled, must_change_password: state.mustChange }, { mfa_verified: state.verified });
      if (url.startsWith("/api/v1/auth/me")) return json(me());
      if (!state.verified && !url.startsWith("/api/v1/auth/mfa") && !url.startsWith("/api/v1/auth/logout")) return err("mfa_required", 403);
      if (url === "/api/v1/auth/mfa/enrol") {
        return json({ issuer: "Sentra DCIM", account: "operator.satu", secret: "ABCD EFGH IJKL MNOP QRST UVWX YZ23 4567", otpauth_uri: "otpauth://totp/x", qr: ["1010101", "0101010", "1010101", "0101010", "1010101", "0101010", "1010101"] });
      }
      if (url === "/api/v1/auth/mfa/enrol/confirm") {
        if (body?.code !== "123456") return err("invalid_mfa_code", 401);
        state.enabled = state.verified = true;
        return json({ recovery_codes: CODES });
      }
      if (url === "/api/v1/auth/mfa/verify") {
        if (body?.code !== "123456" && body?.code !== CODES[0]) return err("invalid_mfa_code", 401);
        state.verified = true;
        return json(null, 204);
      }
      if (url === "/api/v1/auth/mfa/recovery-codes") {
        if (body?.code !== "123456") return err("invalid_mfa_code", 401);
        return json({ recovery_codes: CODES.map((c) => c.replace("a", "z")) });
      }
      if (url === "/api/v1/auth/mfa") return json({ enabled: true, enabled_at: "2026-10-14T01:00:00Z", recovery_codes_remaining: state.remaining });
      if (url.startsWith("/api/v1/ui/bootstrap")) return json(testBootstrap());
      if (url.startsWith("/api/v1/help/index")) return json({ locale: "id", pages: [] });
      return err("not_found", 404);
    }),
  );
  return { state, calls };
}

function renderAt(path: string) {
  window.history.pushState({}, "", path);
  return render(<App queryClient={createQueryClient()} />);
}

describe("MFA", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    window.history.pushState({}, "", "/");
  });

  it("enrols: steps, QR + manual key, wrong then right code, recovery codes once, then the app", async () => {
    const { calls } = fakeApi({ enabled: false });
    renderAt("/help");
    expect(await screen.findByTestId("mfa-enrol-page")).toBeInTheDocument();
    expect(window.location.pathname).toBe("/mfa");
    expect(new URLSearchParams(window.location.search).get("next")).toBe("/help");
    // Nothing behind the guard was requested while pending.
    expect(calls.some((c) => c.url.startsWith("/api/v1/ui/bootstrap"))).toBe(false);

    await userEvent.click(screen.getByTestId("mfa-start"));
    expect(await screen.findByTestId("mfa-qr")).toHaveAccessibleName(tx.mfa.qrLabel.replace("{{account}}", "operator.satu"));
    expect(screen.getByTestId("mfa-secret")).toHaveTextContent("ABCD EFGH IJKL MNOP QRST UVWX YZ23 4567");

    await userEvent.type(screen.getByLabelText(tx.mfa.codeLabel), "000000");
    await userEvent.click(screen.getByTestId("mfa-confirm"));
    expect(await screen.findByTestId("mfa-error")).toHaveTextContent(tx.mfa.invalid);
    expect(screen.getByLabelText(tx.mfa.codeLabel)).toHaveValue("");

    await userEvent.type(screen.getByLabelText(tx.mfa.codeLabel), "123456");
    await userEvent.click(screen.getByTestId("mfa-confirm"));
    const box = await screen.findByTestId("recovery-codes");
    expect(within(box).getAllByTestId("recovery-code")).toHaveLength(10);
    expect(screen.queryByTestId("mfa-secret")).not.toBeInTheDocument();
    expect(screen.getByTestId("recovery-done")).toBeDisabled();
    await userEvent.click(screen.getByTestId("recovery-saved"));
    await userEvent.click(screen.getByTestId("recovery-done"));
    await waitFor(() => expect(window.location.pathname).toBe("/help"));
    expect(screen.queryByText(CODES[0] ?? "")).not.toBeInTheDocument();
  });

  it("verifies with the app code or a recovery code; MFA comes before the forced password change", async () => {
    fakeApi({ enabled: true, mustChange: true });
    renderAt("/dashboard");
    expect(await screen.findByTestId("mfa-verify-page")).toBeInTheDocument();
    await userEvent.click(screen.getByTestId("mfa-toggle-recovery"));
    await userEvent.type(screen.getByLabelText(tx.mfa.recoveryLabel), "wrong-codex");
    await userEvent.click(screen.getByRole("button", { name: tx.mfa.verify }));
    expect(await screen.findByTestId("mfa-error")).toHaveTextContent(tx.mfa.invalid);
    await userEvent.type(screen.getByLabelText(tx.mfa.recoveryLabel), CODES[0] ?? "");
    await userEvent.click(screen.getByRole("button", { name: tx.mfa.verify }));
    expect(await screen.findByTestId("forced-password-page")).toBeInTheDocument();
    expect(new URLSearchParams(window.location.search).get("next")).toBe("/dashboard");
  });

  it("verifies with the app code and continues to the requested page", async () => {
    fakeApi({ enabled: true });
    renderAt("/dashboard");
    await userEvent.type(await screen.findByLabelText(tx.mfa.codeLabel), "123456");
    await userEvent.click(screen.getByRole("button", { name: tx.mfa.verify }));
    expect(await screen.findByTestId("dashboard-empty")).toHaveTextContent(shell.dashboard.emptyTitle);
  });

  it("account security: status, and new recovery codes after a current code", async () => {
    const { state } = fakeApi({ enabled: true });
    state.verified = true;
    state.remaining = 2;
    renderAt("/account/security");
    expect(await screen.findByTestId("mfa-status")).toHaveTextContent(tx.security.enabled);
    expect(screen.getByTestId("recovery-remaining")).toHaveTextContent("2 dari 10");
    expect(screen.getByText(tx.security.recoveryLow)).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(tx.security.title);
    await userEvent.click(screen.getByTestId("regenerate-recovery"));
    const dialog = await screen.findByTestId("regenerate-dialog");
    await userEvent.type(within(dialog).getByLabelText(tx.mfa.codeLabel), "999999");
    await userEvent.click(within(dialog).getByRole("button", { name: tx.security.regenerateSubmit }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(tx.mfa.invalid);
    await userEvent.type(within(dialog).getByLabelText(tx.mfa.codeLabel), "123456");
    await userEvent.click(within(dialog).getByRole("button", { name: tx.security.regenerateSubmit }));
    expect(await within(dialog).findAllByTestId("recovery-code")).toHaveLength(10);
  });
});
