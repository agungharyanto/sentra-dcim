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

export function mockBootstrapFetch(body: unknown, status = 200) {
  const fn = vi.fn(async () => new Response(JSON.stringify(body), { status }));
  vi.stubGlobal("fetch", fn);
  return fn;
}

/** Routes fetch by URL prefix (longest match wins; a handler may return a Response); unknown URLs → 404. */
export function mockApi(routes: Record<string, unknown | ((url: string) => unknown)>) {
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
