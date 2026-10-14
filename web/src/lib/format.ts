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

/** Offset (ms) of timeZone from UTC at the instant t. */
function zoneOffset(t: number, timeZone: string): number {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    hourCycle: "h23",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).formatToParts(new Date(t));
  const n = (type: string) => Number(parts.find((p) => p.type === type)?.value);
  return Date.UTC(n("year"), n("month") - 1, n("day"), n("hour"), n("minute"), n("second")) - Math.floor(t / 1000) * 1000;
}

/**
 * Converts a wall-clock value of an <input type="datetime-local"> ("YYYY-MM-DDTHH:mm", in the display time zone)
 * to an RFC 3339 UTC string for the API. Returns "" for an empty or invalid value.
 */
export function zonedLocalToUtc(local: string, timeZone: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(local);
  if (!m) return "";
  const wall = Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]), Number(m[4]), Number(m[5]));
  let t = wall - zoneOffset(wall, timeZone);
  t = wall - zoneOffset(t, timeZone); // second pass settles DST edges
  return new Date(t).toISOString().replace(".000Z", "Z");
}
