import type { HelpMeta } from "./api";

const fold = (s: string) =>
  s
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase();

export interface SearchHit {
  page: HelpMeta;
  /** Matching subheading, when the title itself did not match. */
  heading?: string;
}

/** Client-side search over page titles and H2/H3 headings (all query words must match). */
export function searchHelp(pages: HelpMeta[], query: string): SearchHit[] {
  const words = fold(query).split(/\s+/).filter(Boolean);
  if (words.length === 0) return pages.map((page) => ({ page }));
  const matches = (text: string) => words.every((w) => fold(text).includes(w));
  const hits: SearchHit[] = [];
  for (const page of pages) {
    if (matches(page.title)) {
      hits.push({ page });
      continue;
    }
    const heading = page.headings.find((h) => matches(`${page.title} ${h.text}`))?.text;
    if (heading) hits.push({ page, heading });
  }
  return hits;
}
