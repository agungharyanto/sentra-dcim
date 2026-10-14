import { createContext, useContext } from "react";
import { useTranslation } from "react-i18next";

import { AuthContext } from "@/features/auth/auth-context";
import {
  type DateFormat,
  type DateTimeOptions,
  formatDate,
  formatDateAndTime,
  formatNumber,
  formatTime,
  type RegionalFormat,
  type TimeFormat,
} from "@/lib/format";

import type { UiBootstrap } from "./api";

export const BootstrapContext = createContext<UiBootstrap | null>(null);

export function useBootstrap(): UiBootstrap {
  const value = useContext(BootstrapContext);
  if (!value) throw new Error("useBootstrap must be used inside <BootstrapGate>");
  return value;
}

/**
 * Effective regional settings: the signed-in user's values from /auth/me (their preference over the system setting),
 * else the system time zone of the bootstrap document with the default formats. The language is the active UI
 * language (kept in sync with the effective language by AuthProvider).
 */
export function useRegional(): RegionalFormat {
  const { i18n } = useTranslation();
  const auth = useContext(AuthContext);
  const boot = useContext(BootstrapContext);
  const locale = i18n.resolvedLanguage ?? i18n.language;
  const r = auth?.me?.regional;
  return {
    locale,
    timeZone: r?.timezone ?? boot?.timezone ?? "UTC",
    timeFormat: (r?.time_format ?? "24h") as TimeFormat,
    dateFormat: (r?.date_format ?? "DD/MM/YYYY") as DateFormat,
  };
}

/** The one place UI code formats dates and times (v0.3.5): values are UTC, display follows useRegional(). */
export function useFormatters() {
  const regional = useRegional();
  return {
    regional,
    locale: regional.locale,
    timezone: regional.timeZone,
    date: (value: Date | string | number) => formatDate(value, regional),
    time: (value: Date | string | number, opts?: DateTimeOptions) => formatTime(value, regional, opts),
    dateTime: (value: Date | string | number, opts?: DateTimeOptions) => formatDateAndTime(value, regional, opts),
    number: (value: number, options?: Intl.NumberFormatOptions) => formatNumber(value, regional.locale, options),
  };
}
