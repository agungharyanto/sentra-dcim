import { getJson, putJson } from "@/lib/api";

import type { Regional } from "@/features/auth/api";

/** Mirrors api/openapi.yaml#/components/schemas/SettingItem. */
export interface SettingItem {
  key: string;
  name: string;
  type: "enum" | "string" | "int" | "bool";
  enum?: string[];
  min?: number;
  max?: number;
  max_length?: number;
  user_overridable: boolean;
  default: unknown;
  value: unknown;
  is_default: boolean;
  updated_at: string | null;
  updated_by: string | null;
}

export interface SettingsGroup {
  group: string;
  items: SettingItem[];
}

/** Mirrors api/openapi.yaml#/components/schemas/Preferences. */
export interface Preferences {
  preferences: Record<string, string | null>;
  effective: Regional;
  system: Regional;
  options: Record<string, string[]>;
}

export const PERM_SETTINGS_MANAGE = "settings.manage";

/** Regional keys, in display order (internal/settings/service). */
export const REGIONAL_NAMES = ["language", "timezone", "time_format", "date_format"] as const;
export type RegionalName = (typeof REGIONAL_NAMES)[number];

export const settingsKey = (group: string) => ["settings", group] as const;
export const preferencesKey = ["me", "preferences"] as const;

export function fetchSettings(group: string, signal?: AbortSignal) {
  return getJson<SettingsGroup>(`/api/v1/settings/${encodeURIComponent(group)}`, signal);
}

/** values: name → new value, or null = reset to the default. */
export function updateSettings(group: string, values: Record<string, unknown>) {
  return putJson<SettingsGroup>(`/api/v1/settings/${encodeURIComponent(group)}`, { values });
}

export function fetchPreferences(signal?: AbortSignal) {
  return getJson<Preferences>("/api/v1/me/preferences", signal);
}

/** values: full key ("regional.timezone") → value, or null = follow the system. */
export function updatePreferences(values: Record<string, string | null>) {
  return putJson<Preferences>("/api/v1/me/preferences", { values });
}

/** IANA time zones offered in pickers (the server validates any IANA name). */
export function timeZones(): string[] {
  const list = typeof Intl.supportedValuesOf === "function" ? Intl.supportedValuesOf("timeZone") : [];
  const out = new Set(["UTC", "Asia/Jakarta", "Asia/Makassar", "Asia/Jayapura", ...list]);
  return [...out];
}
