import { formatDateTime, formatNumber } from "@/lib/format";

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
