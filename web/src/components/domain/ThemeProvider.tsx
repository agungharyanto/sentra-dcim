import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react";

import {
  applyTheme,
  readThemePreference,
  resolveTheme,
  systemPrefersDark,
  type ThemePreference,
  watchSystemTheme,
  writeThemePreference,
} from "@/lib/theme";

import { ThemeContext } from "./theme-context";

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPref] = useState<ThemePreference>(readThemePreference);
  const [prefersDark, setPrefersDark] = useState(systemPrefersDark);
  const resolved = resolveTheme(preference, prefersDark);

  useEffect(() => applyTheme(resolved), [resolved]);
  useEffect(() => (preference === "system" ? watchSystemTheme(setPrefersDark) : undefined), [preference]);

  const setPreference = useCallback((pref: ThemePreference) => {
    writeThemePreference(pref);
    setPref(pref);
  }, []);

  const value = useMemo(() => ({ preference, resolved, setPreference }), [preference, resolved, setPreference]);
  return <ThemeContext value={value}>{children}</ThemeContext>;
}
