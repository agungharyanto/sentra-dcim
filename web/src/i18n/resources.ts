import enAudit from "./locales/en/audit.json";
import enAuth from "./locales/en/auth.json";
import enCommon from "./locales/en/common.json";
import enDs from "./locales/en/ds.json";
import enErrors from "./locales/en/errors.json";
import enFacility from "./locales/en/facility.json";
import enHelp from "./locales/en/help.json";
import enSettings from "./locales/en/settings.json";
import enShell from "./locales/en/shell.json";
import enUsers from "./locales/en/users.json";
import idAudit from "./locales/id/audit.json";
import idAuth from "./locales/id/auth.json";
import idCommon from "./locales/id/common.json";
import idDs from "./locales/id/ds.json";
import idErrors from "./locales/id/errors.json";
import idFacility from "./locales/id/facility.json";
import idHelp from "./locales/id/help.json";
import idSettings from "./locales/id/settings.json";
import idShell from "./locales/id/shell.json";
import idUsers from "./locales/id/users.json";

export const namespaces = ["common", "shell", "errors", "ds", "help", "auth", "users", "audit", "settings", "facility"] as const;
export const supportedLocales = ["id", "en"] as const;
export type Locale = (typeof supportedLocales)[number];

/** Used until the server bootstrap arrives and whenever a key is missing (id is the reference catalog). */
export const fallbackLocale: Locale = "id";

export const resources = {
  id: { common: idCommon, shell: idShell, errors: idErrors, ds: idDs, help: idHelp, auth: idAuth, users: idUsers, audit: idAudit, settings: idSettings, facility: idFacility },
  en: { common: enCommon, shell: enShell, errors: enErrors, ds: enDs, help: enHelp, auth: enAuth, users: enUsers, audit: enAudit, settings: enSettings, facility: enFacility },
} as const;

export function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (supportedLocales as readonly string[]).includes(value);
}
