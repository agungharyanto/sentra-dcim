import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { testBootstrap, testMe } from "@/test/bootstrap";

import { safeNext } from "./auth-context";
import { describeDevice } from "./device";

const auth = resources.id.auth;
const shell = resources.id.shell;

type Handler = (init: RequestInit | undefined, url: string) => Response;
const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(body === undefined ? null : JSON.stringify(body), { status, headers });
const unauth = () => json({ error: { code: "unauthenticated", message: "x" } }, 401);

/** Stateful fake backend: `signedIn` flips with login/logout like the real API. */
function fakeApi(opts: { signedIn?: boolean; mustChange?: boolean; extra?: Record<string, Handler> } = {}) {
  const state = { signedIn: opts.signedIn ?? false, mustChange: opts.mustChange ?? false };
  const me = () => testMe({ must_change_password: state.mustChange });
  const routes: Record<string, Handler> = {
    "GET /api/v1/auth/me": () => (state.signedIn ? json(me()) : unauth()),
    "POST /api/v1/auth/login": (init) => {
      const body = JSON.parse(String(init?.body)) as { username: string; password: string };
      if (body.password !== "benar-sekali-123") return json({ error: { code: "invalid_credentials", message: "x" } }, 401);
      state.signedIn = true;
      return json(me());
    },
    "POST /api/v1/auth/logout": () => {
      state.signedIn = false;
      return json(undefined, 204);
    },
    "POST /api/v1/auth/password": () => {
      state.mustChange = false;
      return json(undefined, 204);
    },
    "GET /api/v1/ui/bootstrap": () => (state.signedIn ? (state.mustChange ? json({ error: { code: "password_change_required" } }, 403) : json(testBootstrap())) : unauth()),
    "GET /api/v1/help/index": () => json({ locale: "id", pages: [] }),
    ...opts.extra,
  };
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const key = `${init?.method ?? "GET"} ${url.split("?")[0]}`;
    const h = routes[key] ?? Object.entries(routes).find(([k]) => key.startsWith(k))?.[1];
    return h ? h(init, url) : json({ error: { code: "not_found" } }, 404);
  });
  vi.stubGlobal("fetch", fn);
  return { fn, state };
}

function renderAt(path: string) {
  window.history.pushState({}, "", path);
  return render(<App queryClient={createQueryClient()} />);
}

describe("auth", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    window.history.pushState({}, "", "/");
  });

  it("redirects to /login with the original path and returns there after signing in", async () => {
    const { fn } = fakeApi();
    renderAt("/help");
    expect(await screen.findByTestId("login-page")).toBeInTheDocument();
    expect(window.location.pathname).toBe("/login");
    expect(new URLSearchParams(window.location.search).get("next")).toBe("/help");
    expect(fn.mock.calls.some(([u]) => String(u).startsWith("/api/v1/ui/bootstrap"))).toBe(false);

    await userEvent.type(screen.getByLabelText(auth.login.username), "operator.satu");
    await userEvent.type(screen.getByLabelText(auth.login.password), "benar-sekali-123");
    await userEvent.click(screen.getByRole("button", { name: auth.login.submit }));
    await waitFor(() => expect(window.location.pathname).toBe("/help"));
    expect(await screen.findByTestId("user-name")).toHaveTextContent("Operator Satu");

    // Login sends the CSRF header and the credentials only in the body.
    const call = fn.mock.calls.find(([u]) => String(u) === "/api/v1/auth/login");
    const headers = call?.[1]?.headers as Record<string, string>;
    expect(headers["X-Sentra-CSRF"]).toBeTruthy();
  });

  it("shows one generic error for wrong credentials and clears the password", async () => {
    fakeApi();
    renderAt("/login");
    await userEvent.type(await screen.findByLabelText(auth.login.username), "operator.satu");
    await userEvent.type(screen.getByLabelText(auth.login.password), "salah");
    await userEvent.click(screen.getByRole("button", { name: auth.login.submit }));
    expect(await screen.findByTestId("login-error")).toHaveTextContent(auth.login.invalid);
    expect(screen.getByLabelText(auth.login.password)).toHaveValue("");
  });

  it("shows the wait time when rate limited", async () => {
    fakeApi({
      extra: {
        "POST /api/v1/auth/login": () => json({ error: { code: "rate_limited" } }, 429, { "Retry-After": "290" }),
      },
    });
    renderAt("/login");
    await userEvent.type(await screen.findByLabelText(auth.login.username), "operator.satu");
    await userEvent.type(screen.getByLabelText(auth.login.password), "apa-saja-123");
    await userEvent.click(screen.getByRole("button", { name: auth.login.submit }));
    expect(await screen.findByTestId("login-error")).toHaveTextContent(auth.login.rateLimited.replace("{{minutes}}", "5"));
  });

  it("forces the password change before the app, then continues", async () => {
    fakeApi({ signedIn: true, mustChange: true });
    renderAt("/dashboard");
    expect(await screen.findByTestId("forced-password-page")).toBeInTheDocument();
    expect(window.location.pathname).toBe("/change-password");

    const form = within(screen.getByTestId("password-form"));
    await userEvent.type(form.getByLabelText(auth.password.current), "sementara-123456");
    await userEvent.type(form.getByLabelText(auth.password.new), "pendek");
    await userEvent.type(form.getByLabelText(auth.password.confirm), "pendek");
    await userEvent.click(form.getByRole("button", { name: auth.password.submit }));
    expect(await screen.findByTestId("password-error")).toHaveTextContent(
      auth.password.policy.replace("{{min}}", "12").replace("{{max}}", "128"),
    );

    await userEvent.clear(form.getByLabelText(auth.password.new));
    await userEvent.clear(form.getByLabelText(auth.password.confirm));
    await userEvent.type(form.getByLabelText(auth.password.new), "kalimat panjang yang baru");
    await userEvent.type(form.getByLabelText(auth.password.confirm), "kalimat panjang yang beda");
    await userEvent.click(form.getByRole("button", { name: auth.password.submit }));
    expect(await screen.findByTestId("password-error")).toHaveTextContent(auth.password.mismatch);

    await userEvent.clear(form.getByLabelText(auth.password.confirm));
    await userEvent.type(form.getByLabelText(auth.password.confirm), "kalimat panjang yang baru");
    await userEvent.click(form.getByRole("button", { name: auth.password.submit }));
    expect(await screen.findByTestId("dashboard-empty")).toBeInTheDocument();
    expect(window.location.pathname).toBe("/dashboard");
  });

  it("signs out from the user menu", async () => {
    fakeApi({ signedIn: true });
    renderAt("/dashboard");
    await userEvent.click(await screen.findByTestId("user-menu"));
    await userEvent.click(await screen.findByTestId("logout"));
    expect(await screen.findByTestId("login-page")).toBeInTheDocument();
    expect(screen.queryByTestId("login-expired")).not.toBeInTheDocument();
  });

  it("an expired session sends the user to the login page with a notice", async () => {
    const { state } = fakeApi({
      signedIn: true,
      extra: { "GET /api/v1/auth/sessions": () => unauth() },
    });
    renderAt("/dashboard");
    expect(await screen.findByTestId("dashboard-empty")).toBeInTheDocument();
    state.signedIn = false;
    await userEvent.click(await screen.findByTestId("user-menu"));
    await userEvent.click(await screen.findByRole("menuitem", { name: auth.sessions.title }));
    expect(await screen.findByTestId("login-expired")).toHaveTextContent(auth.login.expired);
    expect(new URLSearchParams(window.location.search).get("next")).toBe("/account/sessions");
  });

  it("lists own sessions and revokes another one", async () => {
    const other = { ...testMe().session, id: "s-2", current: false, ip: "203.0.113.7", user_agent: "Mozilla/5.0 (Windows NT 10.0) Firefox/140.0" };
    let sessions = [testMe().session, other];
    const revoked: string[] = [];
    fakeApi({
      signedIn: true,
      extra: {
        "GET /api/v1/auth/sessions": () => json({ sessions }),
        "DELETE /api/v1/auth/sessions/": (_init, url) => {
          revoked.push(url.split("/").pop() ?? "");
          sessions = sessions.filter((s) => s.id !== "s-2");
          return json(undefined, 204);
        },
      },
    });
    renderAt("/account/sessions");
    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(auth.sessions.title);
    const rows = await screen.findAllByTestId("session-row");
    expect(rows).toHaveLength(2);
    const [mine, theirs] = rows as [HTMLElement, HTMLElement];
    expect(within(mine).getByText(auth.sessions.current)).toBeInTheDocument();
    expect(within(theirs).getByText("Firefox · Windows")).toBeInTheDocument();
    await userEvent.click(within(theirs).getByRole("button", { name: new RegExp(auth.sessions.revoke) }));
    await waitFor(() => expect(screen.getAllByTestId("session-row")).toHaveLength(1));
    expect(revoked).toEqual(["s-2"]);
    // Breadcrumb names the account group.
    expect(screen.getByRole("navigation", { name: shell.layout.breadcrumb })).toHaveTextContent(auth.account.group);
  });
});

describe("safeNext", () => {
  it.each([
    [null, "/dashboard"],
    ["/help/api", "/help/api"],
    ["//evil.example", "/dashboard"],
    ["/\\evil.example", "/dashboard"],
    ["https://evil.example", "/dashboard"],
    ["/login", "/dashboard"],
  ])("%s → %s", (input, out) => expect(safeNext(input)).toBe(out));
});

describe("describeDevice", () => {
  it("names browser and OS without the full user agent", () => {
    expect(describeDevice("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/141.0 Safari/537.36")).toBe("Chrome · Linux");
    expect(describeDevice("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Safari/604.1")).toBe("Safari · iOS");
    expect(describeDevice("")).toBe("");
  });
});
