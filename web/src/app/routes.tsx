import type { ComponentType } from "react";

import { AuditModule } from "@/features/audit/AuditModule";
import { canSeeMenu } from "@/features/auth/permissions";
import type { MenuItem } from "@/features/bootstrap/api";
import { DashboardPage } from "@/features/dashboard/DashboardPage";
import { FacilityModule } from "@/features/facility/FacilityModule";
import { HelpCenter } from "@/features/help/HelpCenter";
import { SettingsModule } from "@/features/settings/SettingsModule";

import { UsersModule } from "@/features/users/UsersModule";

import { ComingSoonPage, ForbiddenPage, NotConnectedPage } from "./pages/StatusPages";

/**
 * Pages of modules that are available. A registry menu flipped to "available" in the backend must get an entry
 * here in the same version — routes.test.ts fails otherwise (fixture shared with the Go registry tests).
 */
export const moduleRegistry: Record<string, ComponentType> = {
  dashboard: DashboardPage,
  help: HelpCenter,
  users: UsersModule,
  audit: AuditModule,
  settings: SettingsModule,
  facility: FacilityModule,
};

export type RouteKind = "module" | "coming_soon" | "not_connected" | "forbidden";

export interface MenuRoute {
  path: string;
  menu: MenuItem;
  kind: RouteKind;
  element: React.ReactElement;
}

/**
 * One route per registry menu, chosen by permission and status. A menu the user has no permission for renders
 * "Access denied" (the API refuses its data anyway). Unknown/missing pages degrade to ComingSoon at runtime.
 * Paths end in "/*" so modules own their sub-routes (e.g. /help/<slug>).
 */
export function buildRoutes(
  menus: MenuItem[],
  perms: ReadonlySet<string>,
  modules: Record<string, ComponentType> = moduleRegistry,
): MenuRoute[] {
  return menus.map((menu) => {
    if (!canSeeMenu(menu, perms)) {
      return { path: `${menu.path}/*`, menu, kind: "forbidden", element: <ForbiddenPage /> };
    }
    const Page = modules[menu.key];
    if (menu.status === "available" && Page) {
      return { path: `${menu.path}/*`, menu, kind: "module", element: <Page /> };
    }
    if (menu.status === "not_connected") {
      return { path: `${menu.path}/*`, menu, kind: "not_connected", element: <NotConnectedPage menu={menu} /> };
    }
    return { path: `${menu.path}/*`, menu, kind: "coming_soon", element: <ComingSoonPage menu={menu} /> };
  });
}
