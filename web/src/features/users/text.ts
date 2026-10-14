import { useTranslation } from "react-i18next";

import type { resources } from "@/i18n/resources";

type Users = (typeof resources)["id"]["users"];

/**
 * Labels for server-provided role and permission keys. Keys are not known at compile time; unknown ones (e.g. a
 * role added later) fall back to the raw key.
 */
export function useUserText() {
  const { t } = useTranslation("users");
  return {
    roleLabel: (key: string) => t(`roles.${key as keyof Users["roles"]}`, { defaultValue: key }),
    roleDescription: (key: string) => t(`roleDescription.${key as keyof Users["roleDescription"]}`, { defaultValue: "" }),
    permissionLabel: (key: string) =>
      t(`permission.${key.replaceAll(".", "_") as keyof Users["permission"]}`, { defaultValue: key }),
  };
}
