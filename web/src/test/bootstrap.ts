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
