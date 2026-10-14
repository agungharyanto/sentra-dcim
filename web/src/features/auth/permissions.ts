import { useContext, useMemo } from "react";

import type { MenuItem } from "@/features/bootstrap/api";

import { AuthContext } from "./auth-context";

/** Permissions of the signed-in user (empty without a session or outside an AuthProvider, e.g. in tests). */
export function usePermissions(): ReadonlySet<string> {
  const auth = useContext(AuthContext);
  const perms = auth?.me?.user.permissions;
  return useMemo(() => new Set(perms ?? []), [perms]);
}

/** Whether a menu is visible with these permissions. The API enforces the same permission on its routes. */
export function canSeeMenu(menu: MenuItem, perms: ReadonlySet<string>): boolean {
  return !menu.permission || perms.has(menu.permission);
}
