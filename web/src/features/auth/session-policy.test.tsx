import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { allPermissions, testBootstrap, testMe } from "@/test/bootstrap";

import type { AuthMe } from "./api";

const tx = resources.id.auth;
const iso = (msFromNow: number) => new Date(Date.now() + msFromNow).toISOString();

interface Call {
  method: string;
  url: string;
  body: unknown;
}

function api(me: AuthMe, opts: { activityIdle?: number | (() => number); users?: unknown[]; fail?: Record<string, { status: number; body: unknown }>; settings?: Record<string, unknown> } = {}) {
  const calls: Call[] = [];
  const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status });
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      const body = init?.body ? JSON.parse(String(init.body)) : undefined;
      calls.push({ method, url, body });
      const f = opts.fail?.[`${method} ${url}`];
      if (f) return json(f.body, f.status);
      if (url.startsWith("/api/v1/auth/me")) return json(me);
      if (url.startsWith("/api/v1/ui/bootstrap")) return json(testBootstrap());
      if (url === "/api/v1/auth/activity") {
        const idle = typeof opts.activityIdle === "function" ? opts.activityIdle() : (opts.activityIdle ?? 30 * 60_000);
        return json({ session: { ...me.session, idle_expires_at: iso(idle) }, server_time: new Date().toISOString() });
      }
      if (url.startsWith("/api/v1/users/u-9") && method === "GET") return json(opts.users?.[0]);
      if (url.startsWith("/api/v1/roles")) return json({ roles: [], permissions: allPermissions });
      if (url.startsWith("/api/v1/settings/")) {
        const group = url.split("/").pop() ?? "";
        return json(opts.settings?.[group] ?? { group, items: [] });
      }
      return json({ error: { code: "not_found", message: "x" } }, 404);
    }),
  );
  return calls;
}

function renderAt(path: string) {
  window.history.pushState({}, "", path);
  return render(<App queryClient={createQueryClient()} />);
}

describe("session guard (D-069)", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => vi.unstubAllGlobals());

  it("warns 2 minutes before the idle limit and 'Stay signed in' sends the activity signal", async () => {
    // /auth/me (loaded with the page) reports a near deadline: idle for 28 minutes.
    const me = { ...testMe(), session: { ...testMe().session, idle_expires_at: iso(90_000), expires_at: iso(6 * 3600_000) }, server_time: new Date().toISOString() };
    let next = 90_000;
    const calls = api(me, { activityIdle: () => next });
    renderAt("/dashboard");
    const dialog = await screen.findByTestId("session-warning");
    expect(dialog).toHaveTextContent(tx.sessionGuard.title);
    expect(within(dialog).getByTestId("session-countdown")).toHaveTextContent(/^1:[0-3]\d$/);
    expect(within(dialog).queryByRole("button", { name: resources.id.common.actions.close })).not.toBeInTheDocument();
    await waitFor(() => expect(within(dialog).getByTestId("session-stay")).toHaveFocus());
    next = 30 * 60_000; // the user is back: the server extends idle
    await userEvent.click(within(dialog).getByTestId("session-stay"));
    await waitFor(() => expect(screen.queryByTestId("session-warning")).not.toBeInTheDocument());
    expect(calls.filter((c) => c.url === "/api/v1/auth/activity").length).toBeGreaterThanOrEqual(1);
  });

  it("at the absolute limit offers only signing out", async () => {
    const me = { ...testMe(), session: { ...testMe().session, idle_expires_at: iso(25 * 60_000), expires_at: iso(60_000) } };
    api(me, { activityIdle: 25 * 60_000 });
    renderAt("/dashboard");
    const dialog = await screen.findByTestId("session-warning");
    expect(within(dialog).queryByTestId("session-stay")).not.toBeInTheDocument();
    expect(within(dialog).getByTestId("session-logout")).toBeInTheDocument();
  });

  it("kiosk sessions never warn and send no activity", async () => {
    const me = { ...testMe({ kiosk: true }), session: { ...testMe().session, kiosk: true, idle_expires_at: iso(60_000) } };
    const calls = api(me);
    renderAt("/dashboard");
    await screen.findByTestId("dashboard-empty");
    expect(screen.queryByTestId("session-warning")).not.toBeInTheDocument();
    expect(calls.some((c) => c.url === "/api/v1/auth/activity")).toBe(false);
  });

  it("shows the password expiry banner during the warning period", async () => {
    const me = { ...testMe({ password_expires_soon: true, password_expires_at: iso(3 * 86_400_000) }) };
    api(me);
    renderAt("/dashboard");
    const banner = await screen.findByTestId("password-expiry-banner");
    expect(banner).toHaveTextContent("3 hari");
    expect(within(banner).getByRole("link")).toHaveAttribute("href", "/account/password");
  });
});

describe("kiosk card and policy settings", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => vi.unstubAllGlobals());

  it("explains why kiosk mode is refused without Allowed IP", async () => {
    const viewer = {
      id: "u-9", username: "wall", display_name: "Layar NOC", email: null, status: "active", roles: ["viewer"], must_change_password: false,
      locked: false, mfa_enabled: true, kiosk: false, last_login_at: null, created_at: iso(0), updated_at: iso(0),
    };
    const calls = api(testMe(), {
      users: [viewer],
      fail: { "PUT /api/v1/users/u-9/kiosk": { status: 409, body: { error: { code: "kiosk_requires_allowlist", message: "x" } } } },
    });
    renderAt("/users/u-9");
    const card = await screen.findByTestId("kiosk-card");
    await userEvent.click(within(card).getByTestId("kiosk-toggle"));
    await userEvent.click(within(await screen.findByTestId("confirm-dialog")).getByTestId("confirm"));
    expect(await screen.findByText(resources.id.errors.code.kiosk_requires_allowlist)).toBeInTheDocument();
    expect(calls.find((c) => c.method === "PUT" && c.url.endsWith("/kiosk"))?.body).toEqual({ enabled: true });
  });

  it("saves changed policy values as integers and shows field errors", async () => {
    const item = (name: string, value: number, min: number, max: number) => ({
      key: `session.${name}`, name, type: "int", min, max, user_overridable: false, managed: false, default: value, value, is_default: true, updated_at: null, updated_by: null,
    });
    const calls = api(testMe(), {
      settings: { session: { group: "session", items: [item("idle_minutes", 30, 5, 240), item("absolute_hours", 12, 1, 24)] }, password: { group: "password", items: [] } },
      fail: { "PUT /api/v1/settings/password": { status: 422, body: {} } },
    });
    renderAt("/settings/session-password");
    const idle = await screen.findByTestId("policy-idle_minutes");
    await userEvent.clear(idle);
    await userEvent.type(idle, "15");
    await userEvent.click(screen.getByTestId("policy-save-session"));
    await waitFor(() => expect(calls.find((c) => c.method === "PUT")?.body).toEqual({ values: { idle_minutes: 15 } }));
  });
});
