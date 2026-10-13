// WCAG AA contrast for the design tokens, evaluated straight from tokens.css (single source of truth).
//   1.4.3  text (badge/label text, body, links)            >= 4.5:1
//   1.4.11 non-text (StatusDot, badge borders, focus ring)  >= 3:1
import { readFileSync } from "node:fs";
import { join } from "node:path";

// Vitest runs from web/ (css processing is off, so read the file directly).
const css = readFileSync(join(process.cwd(), "src/styles/tokens.css"), "utf8");

function block(selector: string): Record<string, string> {
  const start = css.indexOf(`${selector} {`);
  if (start < 0) throw new Error(`selector ${selector} not found`);
  const body = css.slice(start, css.indexOf("}", start));
  return Object.fromEntries([...body.matchAll(/--([\w-]+):\s*([^;]+);/g)].map((m) => [m[1], (m[2] ?? "").trim()]));
}

const light = block(":root");
const themes = { light, dark: { ...light, ...block(".dark") } };

/** oklch(L C H) → relative luminance (WCAG), via OKLab → linear sRGB (gamut-clipped). */
function luminance(value: string): number {
  const m = /^oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*\)$/.exec(value);
  if (!m) throw new Error(`token must be plain oklch(L C H), got "${value}"`);
  const [L, C, H] = [Number(m[1]), Number(m[2]), (Number(m[3]) * Math.PI) / 180];
  const a = C * Math.cos(H);
  const b = C * Math.sin(H);
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const mm = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  const clip = (v: number) => Math.min(1, Math.max(0, v));
  const r = clip(4.0767416621 * l - 3.3077115913 * mm + 0.2309699292 * s);
  const g = clip(-1.2684380046 * l + 2.6097574011 * mm - 0.3413193965 * s);
  const bl = clip(-0.0041960863 * l - 0.7034186147 * mm + 1.707614701 * s);
  return 0.2126 * r + 0.7152 * g + 0.0722 * bl;
}

function contrast(x: string, y: string): number {
  const [a, b] = [luminance(x), luminance(y)].sort((p, q) => q - p) as [number, number];
  return (a + 0.05) / (b + 0.05);
}

const statuses = ["normal", "warning", "critical", "offline", "maintenance"];
const menus = ["available", "coming-soon", "not-connected"];
const surfaces = ["background", "card", "popover"];

const textPairs: [string, string][] = [
  ...surfaces.flatMap((bg) => [
    [`${bg}-foreground`.replace("background-", ""), bg] as [string, string],
    ["muted-foreground", bg] as [string, string],
    ["link", bg] as [string, string],
  ]),
  ["muted-foreground", "muted"],
  ["accent-foreground", "accent"],
  ["primary-foreground", "primary"],
  ["secondary-foreground", "secondary"],
  ["destructive-foreground", "destructive"],
  ["info-fg", "info-bg"],
  ...statuses.map((s) => [`status-${s}-fg`, `status-${s}-bg`] as [string, string]),
  ...menus.map((s) => [`menu-${s}-fg`, `menu-${s}-bg`] as [string, string]),
];

const nonTextPairs: [string, string][] = [
  ...surfaces.flatMap((bg) => [
    ["ring", bg] as [string, string],
    ["input", bg] as [string, string],
    ...statuses.flatMap((s) => [
      [`status-${s}-solid`, bg] as [string, string],
      [`status-${s}-border`, bg] as [string, string],
    ]),
    ...menus.map((s) => [`menu-${s}-border`, bg] as [string, string]),
  ]),
];

describe.each(Object.entries(themes))("tokens (%s theme)", (_name, tokens) => {
  const get = (k: string) => {
    const v = tokens[k];
    if (!v) throw new Error(`missing token --${k}`);
    return v;
  };

  it.each(textPairs)("text --%s on --%s >= 4.5:1", (fg, bg) => {
    expect(contrast(get(fg), get(bg))).toBeGreaterThanOrEqual(4.5);
  });

  it.each(nonTextPairs)("non-text --%s on --%s >= 3:1", (fg, bg) => {
    expect(contrast(get(fg), get(bg))).toBeGreaterThanOrEqual(3);
  });

  it("status solids are all distinct", () => {
    // Colour is never the only cue (icon + label rule, docs/design-system.md); this guards against copy-paste slips.
    const solids = statuses.map((s) => get(`status-${s}-solid`));
    expect(new Set(solids).size).toBe(statuses.length);
  });
});

it("reference: black on white is 21:1", () => {
  expect(contrast("oklch(0 0 0)", "oklch(1 0 0)")).toBeCloseTo(21, 0);
});
