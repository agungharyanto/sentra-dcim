import { useTranslation } from "react-i18next";

import type { MenuGroup, MenuItem } from "@/features/bootstrap/api";
import type { resources } from "@/i18n/resources";

/** Menus grouped by menu_groups order (menus arrive sorted; groups without menus are dropped). */
export function groupMenus(groups: MenuGroup[], menus: MenuItem[]) {
  return [...groups]
    .sort((a, b) => a.order - b.order)
    .map((group) => ({ group, items: menus.filter((m) => m.group === group.key) }))
    .filter((g) => g.items.length > 0);
}

/** The registry menu a path belongs to (exact path or a sub-path of it). */
export function findMenu(menus: MenuItem[], pathname: string): MenuItem | undefined {
  return menus.find((m) => pathname === m.path || pathname.startsWith(`${m.path}/`));
}

type Shell = (typeof resources)["id"]["shell"];

/**
 * Labels for server-provided registry keys. The keys are not known at compile time, so they are cast to the
 * catalog's key type; i18n:check and the registry fixture test keep catalogs and registry in sync.
 */
export function useMenuText() {
  const { t } = useTranslation("shell");
  return {
    menuLabel: (key: string) => t(`sidebar.${key as keyof Shell["sidebar"]}`),
    groupLabel: (key: string) => t(`groups.${key as keyof Shell["groups"]}`),
    menuDescription: (key: string) => t(`menuDescription.${key as keyof Shell["menuDescription"]}`),
  };
}
