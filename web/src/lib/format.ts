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

/** Regional display settings (GET /api/v1/auth/me → regional; Settings – Regional, v0.3.5). */
export type TimeFormat = "24h" | "12h";
export type DateFormat = "DD/MM/YYYY" | "YYYY-MM-DD" | "DD MMM YYYY";
export const timeFormats: readonly TimeFormat[] = ["24h", "12h"];
export const dateFormats: readonly DateFormat[] = ["DD/MM/YYYY", "YYYY-MM-DD", "DD MMM YYYY"];

export interface RegionalFormat {
  locale: string;
  timeZone: string;
  timeFormat: TimeFormat;
  dateFormat: DateFormat;
}

export interface DateTimeOptions {
  /** Include seconds (default false). */
  seconds?: boolean;
  /** Prefix the short weekday (clock). */
  weekday?: boolean;
  /** Append the time zone abbreviation, e.g. "WIB" (default false). */
  zone?: boolean;
}

// Month names of "DD MMM YYYY": en-US keeps "Sep" (en-GB would give "Sept").
const monthLocales: Record<string, string> = { id: "id-ID", en: "en-US" };

/** Wall-clock parts of value in timeZone. */
function zonedParts(value: Date | string | number, f: RegionalFormat) {
  const date = new Date(value);
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: f.timeZone,
    hourCycle: "h23",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).formatToParts(date);
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? "";
  return { date, year: get("year"), month: get("month"), day: get("day"), hour: get("hour"), minute: get("minute"), second: get("second") };
}

/** Date in the configured pattern: 14/10/2026, 2026-10-14 or 14 Okt 2026. */
export function formatDate(value: Date | string | number, f: RegionalFormat): string {
  const p = zonedParts(value, f);
  switch (f.dateFormat) {
    case "YYYY-MM-DD":
      return `${p.year}-${p.month}-${p.day}`;
    case "DD MMM YYYY": {
      const mon = new Intl.DateTimeFormat(monthLocales[f.locale] ?? intlLocale(f.locale), { timeZone: f.timeZone, month: "short" })
        .format(p.date)
        .replace(".", "");
      return `${p.day} ${mon} ${p.year}`;
    }
    default:
      return `${p.day}/${p.month}/${p.year}`;
  }
}

/** Time in 24 h ("14.30" in Indonesian, "14:30" in English) or 12 h ("2.30 PM" / "2:30 PM"). */
export function formatTime(value: Date | string | number, f: RegionalFormat, opts: DateTimeOptions = {}): string {
  const p = zonedParts(value, f);
  const sep = f.locale === "id" ? "." : ":";
  let h = Number(p.hour);
  let suffix = "";
  let hour = p.hour;
  if (f.timeFormat === "12h") {
    suffix = h < 12 ? " AM" : " PM";
    h = h % 12 || 12;
    hour = String(h);
  }
  let out = `${hour}${sep}${p.minute}${opts.seconds ? `${sep}${p.second}` : ""}${suffix}`;
  if (opts.zone) out += ` ${zoneName(p.date, f)}`;
  return out;
}

/** Short zone name in the UI language ("WIB", "GMT+8"), as Intl knows it. */
export function zoneName(value: Date | string | number, f: RegionalFormat): string {
  const part = new Intl.DateTimeFormat(intlLocale(f.locale), { timeZone: f.timeZone, timeZoneName: "short" })
    .formatToParts(new Date(value))
    .find((x) => x.type === "timeZoneName");
  return part?.value ?? f.timeZone;
}

/** Date and time, e.g. "14/10/2026 14.30" or "Rab, 14/10/2026 2.30.15 PM WIB". */
export function formatDateAndTime(value: Date | string | number, f: RegionalFormat, opts: DateTimeOptions = {}): string {
  const day = opts.weekday
    ? `${new Intl.DateTimeFormat(intlLocale(f.locale), { timeZone: f.timeZone, weekday: "short" }).format(new Date(value))}, `
    : "";
  return `${day}${formatDate(value, f)} ${formatTime(value, f, opts)}`;
}

/** Inverse of zonedLocalToUtc: the wall-clock "YYYY-MM-DDTHH:mm" of a UTC instant in timeZone ("" for "" / invalid). */
export function utcToZonedLocal(utc: string, timeZone: string): string {
  if (!utc || Number.isNaN(new Date(utc).getTime())) return "";
  const p = zonedParts(utc, { locale: "en", timeZone, timeFormat: "24h", dateFormat: "YYYY-MM-DD" });
  return `${p.year}-${p.month}-${p.day}T${p.hour}:${p.minute}`;
}

/** Placeholder pattern of a date-time field, e.g. "DD/MM/YYYY HH.mm" or "YYYY-MM-DD hh:mm AM". */
export function dateTimePattern(f: RegionalFormat): string {
  const sep = f.locale === "id" ? "." : ":";
  return `${f.dateFormat} ${f.timeFormat === "12h" ? `hh${sep}mm AM` : `HH${sep}mm`}`;
}
