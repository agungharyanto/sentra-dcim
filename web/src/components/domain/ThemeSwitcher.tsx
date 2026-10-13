import { Monitor, Moon, Sun } from "lucide-react";
import { useTranslation } from "react-i18next";

import { type ThemePreference, themePreferences } from "@/lib/theme";
import { cn } from "@/lib/utils";

import { useTheme } from "./theme-context";

const icons = { dark: Moon, light: Sun, system: Monitor } as const;

/** Dark / light / follow-system toggle; the choice is stored in this browser. */
export function ThemeSwitcher() {
  const { t } = useTranslation("ds");
  const { preference, setPreference } = useTheme();

  return (
    <div role="group" aria-label={t("theme.label")} className="inline-flex rounded-md border border-input p-0.5">
      {themePreferences.map((pref: ThemePreference) => {
        const Icon = icons[pref];
        return (
          <button
            key={pref}
            type="button"
            aria-pressed={preference === pref}
            title={t(`theme.${pref}`)}
            onClick={() => setPreference(pref)}
            className={cn(
              "inline-flex items-center gap-1.5 rounded px-2.5 py-1 text-sm font-medium transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-ring",
              preference === pref ? "bg-primary text-primary-foreground" : "hover:bg-accent",
            )}
          >
            <Icon className="size-4" aria-hidden />
            <span className="sr-only sm:not-sr-only">{t(`theme.${pref}`)}</span>
          </button>
        );
      })}
    </div>
  );
}
