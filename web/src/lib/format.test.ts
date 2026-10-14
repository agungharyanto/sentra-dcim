import { formatDate, formatDateAndTime, formatDateTime, formatNumber, formatTime, type RegionalFormat, zonedLocalToUtc } from "@/lib/format";

describe("format", () => {
  const utc = "2026-10-13T08:05:00Z";

  it("converts UTC to the server display time zone", () => {
    const opts = { hour: "2-digit", minute: "2-digit" } as const;
    expect(formatDateTime(utc, "id", "Asia/Jakarta", opts)).toBe("15.05");
    expect(formatDateTime(utc, "en", "Asia/Jakarta", opts)).toBe("15:05");
    expect(formatDateTime(utc, "en", "Asia/Makassar", opts)).toBe("16:05");
    expect(formatDateTime(utc, "en", "UTC", opts)).toBe("08:05");
  });

  it("formats dates per locale", () => {
    const opts = { day: "numeric", month: "long", year: "numeric" } as const;
    expect(formatDateTime(utc, "id", "Asia/Jakarta", opts)).toBe("13 Oktober 2026");
    expect(formatDateTime(utc, "en", "Asia/Jakarta", opts)).toBe("13 October 2026");
  });

  it("formats numbers per locale", () => {
    expect(formatNumber(1234.5, "id")).toBe("1.234,5");
    expect(formatNumber(1234.5, "en")).toBe("1,234.5");
  });
});

describe("zonedLocalToUtc", () => {
  it("converts WIB wall-clock time to UTC", () => {
    expect(zonedLocalToUtc("2026-10-14T07:30", "Asia/Jakarta")).toBe("2026-10-14T00:30:00Z");
    expect(zonedLocalToUtc("2026-01-01T00:00", "Asia/Jakarta")).toBe("2025-12-31T17:00:00Z");
    expect(zonedLocalToUtc("2026-10-14T07:30", "UTC")).toBe("2026-10-14T07:30:00Z");
  });
  it("rejects malformed input", () => {
    expect(zonedLocalToUtc("", "Asia/Jakarta")).toBe("");
    expect(zonedLocalToUtc("yesterday", "Asia/Jakarta")).toBe("");
  });
});

describe("regional formats (every combination)", () => {
  const t = "2026-10-14T07:05:09Z"; // 14.05.09 WIB, 15.05.09 WITA
  const f = (o: Partial<RegionalFormat>): RegionalFormat => ({ locale: "id", timeZone: "Asia/Jakarta", timeFormat: "24h", dateFormat: "DD/MM/YYYY", ...o });

  it.each([
    ["id", "DD/MM/YYYY", "14/10/2026"],
    ["id", "YYYY-MM-DD", "2026-10-14"],
    ["id", "DD MMM YYYY", "14 Okt 2026"],
    ["en", "DD/MM/YYYY", "14/10/2026"],
    ["en", "YYYY-MM-DD", "2026-10-14"],
    ["en", "DD MMM YYYY", "14 Oct 2026"],
  ] as const)("date %s %s", (locale, dateFormat, want) => {
    expect(formatDate(t, f({ locale, dateFormat }))).toBe(want);
  });

  it.each([
    ["id", "24h", false, "14.05"],
    ["id", "24h", true, "14.05.09"],
    ["id", "12h", false, "2.05 PM"],
    ["en", "24h", false, "14:05"],
    ["en", "12h", true, "2:05:09 PM"],
  ] as const)("time %s %s seconds=%s", (locale, timeFormat, seconds, want) => {
    expect(formatTime(t, f({ locale, timeFormat }), { seconds })).toBe(want);
  });

  it("follows the time zone, midnight and noon in 12 h", () => {
    expect(formatDateAndTime(t, f({ timeZone: "Asia/Makassar" }))).toBe("14/10/2026 15.05");
    expect(formatDateAndTime("2026-10-13T17:30:00Z", f({ timeFormat: "12h", locale: "en" }))).toBe("14/10/2026 12:30 AM");
    expect(formatTime("2026-10-14T05:00:00Z", f({ timeFormat: "12h", locale: "en" }))).toBe("12:00 PM");
    expect(formatDate("2026-10-13T17:30:00Z", f({ timeZone: "UTC" }))).toBe("13/10/2026");
  });

  it("adds weekday and zone", () => {
    expect(formatDateAndTime(t, f({}), { weekday: true, zone: true, seconds: true })).toBe("Rab, 14/10/2026 14.05.09 WIB");
  });
});
