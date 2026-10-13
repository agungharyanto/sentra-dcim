// Display formatting only: values arrive and are stored as UTC; the server decides the display time zone.

const intlLocales: Record<string, string> = { id: "id-ID", en: "en-GB" };

export function intlLocale(locale: string): string {
  return intlLocales[locale] ?? locale;
}

export function formatDateTime(
  value: Date | string | number,
  locale: string,
  timeZone: string,
  options: Intl.DateTimeFormatOptions = { dateStyle: "medium", timeStyle: "medium" },
): string {
  // 24-hour clock until per-user format arrives with Settings – Regional (v0.3.5).
  return new Intl.DateTimeFormat(intlLocale(locale), { hourCycle: "h23", ...options, timeZone }).format(
    new Date(value),
  );
}

export function formatNumber(value: number, locale: string, options?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(intlLocale(locale), options).format(value);
}
