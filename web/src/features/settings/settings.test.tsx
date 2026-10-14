import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import type { Regional } from "@/features/auth/api";
import i18n, { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { allPermissions, testBootstrap, testMe } from "@/test/bootstrap";

import type { Preferences, SettingItem } from "./api";

const tx = resources.id.settings;

const item = (name: string, value: string, def: string, o: Partial<SettingItem> = {}): SettingItem => ({
  key: `regional.${name}`,
  name,
  type: name === "timezone" ? "string" : "enum",
  user_overridable: true,
  default: def,
  value,
  is_default: value === def,
  updated_at: value === def ? null : "2026-10-14T02:00:00Z",
  updated_by: null,
  ...o,
});

interface Call {
  method: string;
  url: string;
  body: unknown;
}

function api(opts: { perms?: string[]; regional?: Regional; fail?: { status: number; body: unknown } } = {}) {
  const calls: Call[] = [];
  let sys = { language: "id", timezone: "Asia/Jakarta", time_format: "24h", date_format: "DD/MM/YYYY" } as Regional;
  let prefs: Record<string, string | null> = {
    "regional.language": null,
    "regional.timezone": null,
    "regional.time_format": null,
    "regional.date_format": null,
  };
  const group = () => ({
    group: "regional",
    items: [
      item("language", sys.language, "id"),
      item("timezone", sys.timezone, "Asia/Jakarta"),
      item("time_format", sys.time_format, "24h"),
      item("date_format", sys.date_format, "DD/MM/YYYY"),
    ],
  });
  const effective = (): Regional => ({
    language: prefs["regional.language"] ?? sys.language,
    timezone: prefs["regional.timezone"] ?? sys.timezone,
    time_format: (prefs["regional.time_format"] ?? sys.time_format) as Regional["time_format"],
    date_format: (prefs["regional.date_format"] ?? sys.date_format) as Regional["date_format"],
  });
  const prefsDoc = (): Preferences => ({ preferences: prefs, effective: effective(), system: sys, options: {} });
  const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status });
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      const body = init?.body ? JSON.parse(String(init.body)) : undefined;
      calls.push({ method, url, body });
      if (url.startsWith("/api/v1/auth/me")) return json({ ...testMe({ permissions: opts.perms ?? allPermissions }), regional: opts.regional ?? effective() });
      if (url.startsWith("/api/v1/ui/bootstrap")) return json(testBootstrap({ timezone: sys.timezone }));
      if (url === "/api/v1/settings/regional") {
        if (method === "PUT") {
          if (opts.fail) return json(opts.fail.body, opts.fail.status);
          for (const [k, v] of Object.entries(body.values as Record<string, string | null>)) {
            const defaults: Record<string, string> = { language: "id", timezone: "Asia/Jakarta", time_format: "24h", date_format: "DD/MM/YYYY" };
            sys = { ...sys, [k]: v ?? defaults[k] };
          }
        }
        return json(group());
      }
      if (url === "/api/v1/me/preferences") {
        if (method === "PUT") prefs = { ...prefs, ...(body.values as Record<string, string | null>) };
        return json(prefsDoc());
      }
      if (url.startsWith("/api/v1/auth/sessions")) return json({ sessions: [testMe().session] });
      return json({ events: [], next_before: null, limit: 50 });
    }),
  );
  return calls;
}

function renderAt(path: string) {
  window.history.pushState({}, "", path);
  return render(<App queryClient={createQueryClient()} />);
}

describe("settings › regional", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => vi.unstubAllGlobals());

  it("opens /settings on Regional and saves only the changed fields", async () => {
    const calls = api();
    renderAt("/settings");
    const form = await screen.findByTestId("settings-regional");
    expect(window.location.pathname).toBe("/settings/regional");
    await userEvent.selectOptions(within(form).getByTestId("regional-timezone"), "Asia/Makassar");
    await userEvent.selectOptions(within(form).getByTestId("regional-time_format"), "12h");
    expect(screen.getByTestId("preview-datetime")).toHaveTextContent(/(AM|PM) WITA$/);
    await userEvent.click(screen.getByTestId("settings-save"));
    await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
    const put = calls.find((c) => c.method === "PUT");
    expect(put?.body).toEqual({ values: { timezone: "Asia/Makassar", time_format: "12h" } });
    expect(await screen.findByText(tx.regional.saved)).toBeInTheDocument();
    // Changed fields show their default and a reset button; reset sends null.
    await userEvent.click(await screen.findByTestId("reset-timezone"));
    await waitFor(() => expect(calls.filter((c) => c.method === "PUT")).toHaveLength(2));
    expect(calls.filter((c) => c.method === "PUT")[1]?.body).toEqual({ values: { timezone: null } });
  });

  it("shows the server's validation error on the field", async () => {
    api({ fail: { status: 422, body: { error: { code: "validation_failed", message: "unknown time zone", field: "timezone" } } } });
    renderAt("/settings/regional");
    const form = await screen.findByTestId("settings-regional");
    await userEvent.selectOptions(within(form).getByTestId("regional-timezone"), "UTC");
    await userEvent.click(screen.getByTestId("settings-save"));
    expect(await within(form).findByRole("alert")).toHaveTextContent(resources.id.errors.code.validation_failed);
    expect(within(form).getByTestId("regional-timezone")).toHaveAttribute("aria-invalid", "true");
  });

  it("denies the page without settings.manage", async () => {
    api({ perms: ["audit.read"] });
    renderAt("/settings/regional");
    expect(await screen.findByText(resources.id.shell.pages.forbidden.title)).toBeInTheDocument();
  });
});

describe("account › preferences", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => vi.unstubAllGlobals());

  it("offers 'follow system' with the system value and saves own values / null", async () => {
    const calls = api();
    renderAt("/account/preferences");
    const page = await screen.findByTestId("preferences-page");
    const tz = within(page).getByTestId("regional-timezone");
    expect(tz).toHaveValue("");
    expect(within(tz).getByRole("option", { name: "Ikuti sistem (Asia/Jakarta)" })).toBeInTheDocument();
    await userEvent.selectOptions(tz, "UTC");
    await userEvent.selectOptions(within(page).getByTestId("regional-date_format"), "YYYY-MM-DD");
    await userEvent.click(screen.getByTestId("preferences-save"));
    await waitFor(() => expect(calls.some((c) => c.method === "PUT")).toBe(true));
    expect(calls.find((c) => c.method === "PUT")?.body).toEqual({ values: { "regional.timezone": "UTC", "regional.date_format": "YYYY-MM-DD" } });
    expect(await screen.findByText(tx.preferences.saved)).toBeInTheDocument();
    // Back to "follow system" sends null.
    await userEvent.selectOptions(await within(await screen.findByTestId("preferences-page")).findByTestId("regional-timezone"), "");
    await userEvent.click(screen.getByTestId("preferences-save"));
    await waitFor(() => expect(calls.filter((c) => c.method === "PUT")).toHaveLength(2));
    expect(calls.filter((c) => c.method === "PUT")[1]?.body).toEqual({ values: { "regional.timezone": null } });
  });

  it("effective values from /auth/me drive every date in the UI (own preference over system)", async () => {
    api({ regional: { language: "en", timezone: "UTC", time_format: "12h", date_format: "YYYY-MM-DD" } });
    renderAt("/account/sessions");
    // testMe session: created 2026-10-14T01:00:00Z → UTC, 12 h, ISO date; language follows the preference.
    expect((await screen.findAllByText(/2026-10-14 1:00 AM/)).length).toBeGreaterThan(0);
    await waitFor(() => expect(i18n.resolvedLanguage).toBe("en"));
  });
});
