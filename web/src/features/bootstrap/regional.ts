import { createContext, useContext } from "react";
import { useTranslation } from "react-i18next";

import { formatDateTime, formatNumber } from "@/lib/format";

import type { UiBootstrap } from "./api";

export const BootstrapContext = createContext<UiBootstrap | null>(null);

export function useBootstrap(): UiBootstrap {
  const value = useContext(BootstrapContext);
  if (!value) throw new Error("useBootstrap must be used inside <BootstrapGate>");
  return value;
}

/** Formatters bound to the active UI language and the server display time zone. */
export function useFormatters() {
  const { i18n } = useTranslation();
  const { timezone } = useBootstrap();
  const locale = i18n.resolvedLanguage ?? i18n.language;
  return {
    locale,
    timezone,
    dateTime: (value: Date | string | number, options?: Intl.DateTimeFormatOptions) =>
      formatDateTime(value, locale, timezone, options),
    number: (value: number, options?: Intl.NumberFormatOptions) => formatNumber(value, locale, options),
  };
}
