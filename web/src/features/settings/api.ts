import { deleteJson, getJson, patchJson, postJson, putJson } from "@/lib/api";

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

/** Mirrors api/openapi.yaml#/components/schemas/AllowlistEntry. */
export interface AllowlistEntry {
  id: string;
  cidr: string;
  label: string;
  created_at: string;
  created_by: string | null;
  updated_at: string;
}

export type AllowlistMode = "off" | "monitor" | "enforce";
export const ALLOWLIST_MODES: readonly AllowlistMode[] = ["off", "monitor", "enforce"];

/** Mirrors api/openapi.yaml#/components/schemas/AllowlistState. */
export interface AllowlistState {
  mode: AllowlistMode;
  pending_until: string | null;
  entries: AllowlistEntry[];
  your_ip: string;
  your_ip_in_list: boolean;
  always_public: string[];
  confirm_window_seconds: number;
  server_time: string;
}

export const allowlistKey = ["settings", "allowed-ips"] as const;

export function fetchAllowlist(signal?: AbortSignal) {
  return getJson<AllowlistState>("/api/v1/settings/allowed-ips", signal);
}

export function addAllowed(cidr: string, label: string) {
  return postJson<AllowlistState>("/api/v1/settings/allowed-ips", { cidr, label });
}

export function relabelAllowed(id: string, label: string) {
  return patchJson<AllowlistState>(`/api/v1/settings/allowed-ips/${encodeURIComponent(id)}`, { label });
}

export function removeAllowed(id: string) {
  return deleteJson<AllowlistState>(`/api/v1/settings/allowed-ips/${encodeURIComponent(id)}`);
}

export function setAllowlistMode(mode: AllowlistMode) {
  return putJson<AllowlistState>("/api/v1/settings/allowed-ips/mode", { mode });
}

export function confirmEnforce() {
  return postJson<AllowlistState>("/api/v1/settings/allowed-ips/confirm");
}
