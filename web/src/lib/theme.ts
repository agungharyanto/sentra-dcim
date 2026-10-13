// Theme preference: dark (default) | light | system. Stored per browser until per-user settings (v0.3.5).
// No inline script in index.html (strict CSP): <html class="dark"> is static, so a stored "light" choice shows a
// short dark→light switch on load — accepted, see docs/design-system.md.

export const themePreferences = ["dark", "light", "system"] as const;
export type ThemePreference = (typeof themePreferences)[number];
export type ResolvedTheme = "dark" | "light";

export const DEFAULT_THEME: ThemePreference = "dark";
const STORAGE_KEY = "sentra.theme";
const DARK_QUERY = "(prefers-color-scheme: dark)";

export function isThemePreference(value: unknown): value is ThemePreference {
  return typeof value === "string" && (themePreferences as readonly string[]).includes(value);
}

/** Stored preference; anything unreadable or invalid falls back to dark. */
export function readThemePreference(): ThemePreference {
  try {
    const value = window.localStorage.getItem(STORAGE_KEY);
    return isThemePreference(value) ? value : DEFAULT_THEME;
  } catch {
    return DEFAULT_THEME;
  }
}

export function writeThemePreference(pref: ThemePreference): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, pref);
  } catch {
    // Storage disabled (private mode, policy): the choice still applies to this page.
  }
}

export function systemPrefersDark(): boolean {
  try {
    return window.matchMedia(DARK_QUERY).matches;
  } catch {
    return true; // no media query support: keep the dark default
  }
}

export function resolveTheme(pref: ThemePreference, prefersDark = systemPrefersDark()): ResolvedTheme {
  if (pref === "system") return prefersDark ? "dark" : "light";
  return pref;
}

export function applyTheme(theme: ResolvedTheme): void {
  document.documentElement.classList.toggle("dark", theme === "dark");
}

/** Subscribes to OS theme changes; returns the unsubscribe function. */
export function watchSystemTheme(onChange: (prefersDark: boolean) => void): () => void {
  try {
    const mql = window.matchMedia(DARK_QUERY);
    const listener = (e: MediaQueryListEvent) => onChange(e.matches);
    mql.addEventListener("change", listener);
    return () => mql.removeEventListener("change", listener);
  } catch {
    return () => {};
  }
}
