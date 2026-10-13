import { helpIndex } from "@/test/bootstrap";

import type { HelpMeta } from "./api";
import { helpPathForMenu } from "./api";
import { searchHelp } from "./search";

const pages = helpIndex("id").pages as HelpMeta[];

describe("help search", () => {
  it("lists every page for an empty query", () => {
    expect(searchHelp(pages, "  ")).toHaveLength(pages.length);
  });

  it("matches titles case- and diacritic-insensitively", () => {
    expect(searchHelp(pages, "DASBOR").map((h) => h.page.slug)).toEqual(["dashboard"]);
  });

  it("matches subheadings and reports which one", () => {
    const hits = searchHelp(pages, "tema jam");
    expect(hits).toEqual([{ page: expect.objectContaining({ slug: "getting-started" }), heading: "Bahasa, tema, dan jam" }]);
  });

  it("returns nothing when no word matches", () => {
    expect(searchHelp(pages, "genset")).toEqual([]);
  });
});

describe("help button target", () => {
  it("opens the page whose front matter names the active menu", () => {
    expect(helpPathForMenu(pages, "dashboard")).toBe("/help/dashboard");
  });
  it("falls back to the Help Center home", () => {
    expect(helpPathForMenu(pages, "alarms")).toBe("/help");
    expect(helpPathForMenu(pages, undefined)).toBe("/help");
    expect(helpPathForMenu(undefined, "dashboard")).toBe("/help");
    expect(helpPathForMenu(pages, "help")).toBe("/help");
  });
});
