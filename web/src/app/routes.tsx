import type { ComponentType } from "react";

import type { MenuItem } from "@/features/bootstrap/api";
import { DashboardPage } from "@/features/dashboard/DashboardPage";

import { ComingSoonPage, NotConnectedPage } from "./pages/StatusPages";

/**
 * Pages of modules that are available. A registry menu flipped to "available" in the backend must get an entry
 * here in the same version — routes.test.ts fails otherwise (fixture shared with the Go registry tests).
 */
export const moduleRegistry: Record<string, ComponentType> = {
  dashboard: DashboardPage,
};

export type RouteKind = "module" | "coming_soon" | "not_connected";

export interface MenuRoute {
  path: string;
  menu: MenuItem;
  kind: RouteKind;
  element: React.ReactElement;
}

/** One route per registry menu, chosen by status. Unknown/missing pages degrade to ComingSoon at runtime. */
export function buildRoutes(menus: MenuItem[], modules: Record<string, ComponentType> = moduleRegistry): MenuRoute[] {
  return menus.map((menu) => {
    const Page = modules[menu.key];
    if (menu.status === "available" && Page) {
      return { path: menu.path, menu, kind: "module", element: <Page /> };
    }
    if (menu.status === "not_connected") {
      return { path: menu.path, menu, kind: "not_connected", element: <NotConnectedPage menu={menu} /> };
    }
    return { path: menu.path, menu, kind: "coming_soon", element: <ComingSoonPage menu={menu} /> };
  });
}
