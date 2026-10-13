import enCommon from "./locales/en/common.json";
import enDs from "./locales/en/ds.json";
import enErrors from "./locales/en/errors.json";
import enShell from "./locales/en/shell.json";
import idCommon from "./locales/id/common.json";
import idDs from "./locales/id/ds.json";
import idErrors from "./locales/id/errors.json";
import idShell from "./locales/id/shell.json";

export const namespaces = ["common", "shell", "errors", "ds"] as const;
export const supportedLocales = ["id", "en"] as const;
export type Locale = (typeof supportedLocales)[number];

/** Used until the server bootstrap arrives and whenever a key is missing (id is the reference catalog). */
export const fallbackLocale: Locale = "id";

export const resources = {
  id: { common: idCommon, shell: idShell, errors: idErrors, ds: idDs },
  en: { common: enCommon, shell: enShell, errors: enErrors, ds: enDs },
} as const;

export function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (supportedLocales as readonly string[]).includes(value);
}
