import { getJson } from "@/lib/api";

export type MenuStatus = "available" | "coming_soon" | "not_connected";

/** Mirrors api/openapi.yaml#/components/schemas/MenuGroup. Label: shell:groups.<key>. */
export interface MenuGroup {
  key: string;
  order: number;
}

/** Mirrors api/openapi.yaml#/components/schemas/MenuItem. Labels are translated from `key` (shell:sidebar.<key>). */
export interface MenuItem {
  key: string;
  path: string;
  icon: string;
  status: MenuStatus;
  group: string;
  order: number;
}

/** Mirrors api/openapi.yaml#/components/schemas/UiBootstrap. */
export interface UiBootstrap {
  app_name: string;
  version: string;
  default_locale: string;
  supported_locales: string[];
  timezone: string;
  menu_groups: MenuGroup[];
  menus: MenuItem[];
}

export const bootstrapQueryKey = ["ui", "bootstrap"] as const;

export function fetchBootstrap(signal?: AbortSignal) {
  return getJson<UiBootstrap>("/api/v1/ui/bootstrap", signal);
}
