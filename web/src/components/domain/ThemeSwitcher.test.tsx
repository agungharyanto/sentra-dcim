import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ThemeProvider } from "@/components/domain/ThemeProvider";
import { ThemeSwitcher } from "@/components/domain/ThemeSwitcher";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { readThemePreference, resolveTheme } from "@/lib/theme";

const ds = resources.id.ds;

/** Controllable prefers-color-scheme media query. */
function mockMatchMedia(initialDark: boolean) {
  const listeners = new Set<(e: MediaQueryListEvent) => void>();
  const mql = {
    matches: initialDark,
    addEventListener: (_: string, l: (e: MediaQueryListEvent) => void) => listeners.add(l),
    removeEventListener: (_: string, l: (e: MediaQueryListEvent) => void) => listeners.delete(l),
  };
  vi.stubGlobal("matchMedia", () => mql);
  return (dark: boolean) => {
    mql.matches = dark;
    listeners.forEach((l) => l({ matches: dark } as MediaQueryListEvent));
  };
}

function renderSwitcher() {
  return render(
    <ThemeProvider>
      <ThemeSwitcher />
    </ThemeProvider>,
  );
}

const html = () => document.documentElement;

describe("theme", () => {
  beforeEach(async () => {
    await initI18n("id");
    html().classList.add("dark");
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("defaults to dark and stores the user's choice", async () => {
    mockMatchMedia(true);
    renderSwitcher();
    expect(screen.getByRole("button", { name: ds.theme.dark })).toHaveAttribute("aria-pressed", "true");
    expect(html()).toHaveClass("dark");

    await userEvent.click(screen.getByRole("button", { name: ds.theme.light }));
    expect(html()).not.toHaveClass("dark");
    expect(window.localStorage.getItem("sentra.theme")).toBe("light");
    expect(readThemePreference()).toBe("light");
  });

  it("follows the operating system when set to system", async () => {
    const setSystemDark = mockMatchMedia(false);
    renderSwitcher();
    await userEvent.click(screen.getByRole("button", { name: ds.theme.system }));
    expect(html()).not.toHaveClass("dark");
    act(() => setSystemDark(true));
    expect(html()).toHaveClass("dark");
  });

  it("falls back to dark when the stored value is invalid", () => {
    window.localStorage.setItem("sentra.theme", "purple");
    expect(readThemePreference()).toBe("dark");
  });

  it("falls back to dark when storage throws and still switches for this page", async () => {
    mockMatchMedia(true);
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("denied");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(readThemePreference()).toBe("dark");
    renderSwitcher();
    await userEvent.click(screen.getByRole("button", { name: ds.theme.light }));
    expect(html()).not.toHaveClass("dark");
  });

  it("resolves system without media query support to dark", () => {
    vi.stubGlobal("matchMedia", undefined);
    expect(resolveTheme("system")).toBe("dark");
  });
});
