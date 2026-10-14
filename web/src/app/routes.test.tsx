import type { MenuItem } from "@/features/bootstrap/api";
import registry from "@/test/fixtures/ui-registry.json";

import { buildRoutes, moduleRegistry } from "./routes";

const menus = registry.menus as MenuItem[];

describe("registry ↔ pages", () => {
  // Fails when the backend marks a menu available without a page in moduleRegistry (fixture: go test -update).
  it.each(menus.filter((m) => m.status === "available").map((m) => [m.key]))(
    "available menu %s has a page in moduleRegistry",
    (key) => {
      expect(moduleRegistry[key], `add "${key}" to moduleRegistry in src/app/routes.tsx`).toBeDefined();
    },
  );

  it("fixture holds the full registry (14 menus in 4 groups)", () => {
    expect(menus).toHaveLength(14);
    expect(registry.menu_groups.map((g) => g.key)).toEqual(["monitoring", "infrastructure", "operations", "system"]);
  });
});

describe("buildRoutes", () => {
  const menu = (key: string, status: MenuItem["status"], permission = ""): MenuItem => ({
    key,
    path: `/${key}`,
    icon: "circle",
    status,
    group: "system",
    order: 10,
    permission,
  });
  const none = new Set<string>();

  it("maps each status to its standard page", () => {
    const routes = buildRoutes(
      [menu("dashboard", "available"), menu("alarms", "coming_soon"), menu("power", "not_connected")],
      none,
    );
    expect(routes.map((r) => [r.path, r.kind])).toEqual([
      ["/dashboard/*", "module"],
      ["/alarms/*", "coming_soon"],
      ["/power/*", "not_connected"],
    ]);
  });

  it("degrades an available menu without a page to ComingSoon at runtime", () => {
    const [route] = buildRoutes([menu("brand-new", "available")], none, {});
    expect(route?.kind).toBe("coming_soon");
  });

  it("creates one route per registry menu", () => {
    // "/*": modules own their sub-routes (e.g. /help/<slug>).
    expect(buildRoutes(menus, none).map((r) => r.path)).toEqual(menus.map((m) => `${m.path}/*`));
  });

  it("renders Access denied for a menu the user has no permission for", () => {
    const users = menu("users", "available", "users.read");
    expect(buildRoutes([users], none)[0]?.kind).toBe("forbidden");
    expect(buildRoutes([users], new Set(["users.read"]))[0]?.kind).toBe("module");
    // A permission-free menu is open to every signed-in user.
    expect(buildRoutes([menu("dashboard", "available")], none)[0]?.kind).toBe("module");
  });
});
