import { useTranslation } from "react-i18next";

import { isLocale, type Locale, setLocale, supportedLocales } from "@/i18n";
import { cn } from "@/lib/utils";

/**
 * ID/EN toggle. Offers only locales that both the server and the bundled catalogs support. onSelect (signed in)
 * additionally saves the choice as the user's language preference; without it the choice stays in this browser.
 */
export function LanguageSwitcher({ serverLocales, onSelect }: { serverLocales: readonly string[]; onSelect?: ((lng: Locale) => void) | undefined }) {
  const { t, i18n } = useTranslation();
  const locales = supportedLocales.filter((l: Locale) => serverLocales.filter(isLocale).includes(l));
  const active = i18n.resolvedLanguage;

  return (
    <div role="group" aria-label={t("language.label")} className="inline-flex rounded-md border border-input p-0.5">
      {locales.map((lng) => (
        <button
          key={lng}
          type="button"
          lang={lng}
          aria-pressed={active === lng}
          aria-label={t("language.switchTo", { language: t(`language.${lng}`) })}
          onClick={() => {
            void setLocale(lng);
            onSelect?.(lng);
          }}
          className={cn(
            "rounded px-2.5 py-1 text-sm font-medium transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-ring",
            active === lng ? "bg-primary text-primary-foreground" : "hover:bg-accent",
          )}
        >
          {t(`language.short.${lng}`)}
        </button>
      ))}
    </div>
  );
}
