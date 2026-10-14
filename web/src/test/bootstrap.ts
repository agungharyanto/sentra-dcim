import type { AuthMe } from "@/features/auth/api";
import type { UiBootstrap } from "@/features/bootstrap/api";

import registry from "./fixtures/ui-registry.json";

/** Bootstrap document built from the real backend registry (fixture written by `go test ./internal/ui/service`). */
export function testBootstrap(overrides: Partial<UiBootstrap> = {}): UiBootstrap {
  return {
    app_name: "Sentra DCIM",
    version: "v0.2.3",
    default_locale: "id",
    supported_locales: ["id", "en"],
    timezone: "Asia/Jakarta",
    menu_groups: registry.menu_groups,
    menus: registry.menus as UiBootstrap["menus"],
    ...overrides,
  };
}

/** Every permission (Admin); tests narrow it with testMe({ roles, permissions }). */
export const allPermissions = ["roles.read", "system.version.read", "ui.design_system", "users.manage", "users.read"];

/** Signed-in user for component tests (GET /api/v1/auth/me); an Admin unless overridden. */
export function testMe(overrides: Partial<AuthMe["user"]> = {}, session: Partial<AuthMe["session"]> = {}): AuthMe {
  return {
    user: {
      id: "u-1",
      username: "operator.satu",
      display_name: "Operator Satu",
      must_change_password: false,
      roles: ["admin"],
      permissions: allPermissions,
      mfa_enabled: true,
      ...overrides,
    },
    session: {
      id: "s-1",
      created_at: "2026-10-14T01:00:00Z",
      last_seen_at: "2026-10-14T01:05:00Z",
      expires_at: "2026-10-14T13:00:00Z",
      idle_expires_at: "2026-10-14T01:35:00Z",
      ip: null,
      user_agent: "",
      current: true,
      mfa_verified: true,
      ...session,
    },
  };
}

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });

/** Every request answers `body`, except /api/v1/auth/me (signed in as testMe()). */
export function mockBootstrapFetch(body: unknown, status = 200) {
  const fn = vi.fn(async (input: RequestInfo | URL) =>
    String(input).startsWith("/api/v1/auth/me") ? json(testMe()) : json(body, status),
  );
  vi.stubGlobal("fetch", fn);
  return fn;
}

/**
 * Routes fetch by URL prefix (longest match wins; a handler may return a Response); unknown URLs → 404.
 * /api/v1/auth/me defaults to a signed-in testMe().
 */
export function mockApi(input: Record<string, unknown | ((url: string) => unknown)>) {
  const routes: Record<string, unknown> = { "/api/v1/auth/me": testMe(), ...input };
  const keys = Object.keys(routes).sort((a, b) => b.length - a.length);
  const fn = vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input);
    const key = keys.find((k) => url.startsWith(k));
    if (!key) return new Response(JSON.stringify({ error: { code: "not_found", message: "x" } }), { status: 404 });
    const value = routes[key];
    const body = typeof value === "function" ? (value as (u: string) => unknown)(url) : value;
    return body instanceof Response ? body : new Response(JSON.stringify(body), { status: 200 });
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}

export const helpIndex = (locale: string) => ({
  locale,
  pages: [
    { slug: "help", title: locale === "en" ? "Help Center" : "Pusat Bantuan", audience: "user", menu: "help", order: 10, headings: [] },
    {
      slug: "getting-started",
      title: locale === "en" ? "Getting started" : "Memulai",
      audience: "user",
      order: 20,
      headings: [{ level: 2, text: locale === "en" ? "Keyboard" : "Keyboard" }, { level: 2, text: locale === "en" ? "Language, theme and clock" : "Bahasa, tema, dan jam" }],
    },
    { slug: "dashboard", title: locale === "en" ? "Dashboard" : "Dasbor", audience: "user", menu: "dashboard", order: 30, headings: [] },
    { slug: "api", title: locale === "en" ? "API documentation" : "Dokumentasi API", audience: "admin", order: 90, headings: [] },
  ],
});
