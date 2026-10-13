import { useTranslation } from "react-i18next";

import { isLocale, type Locale, setLocale, supportedLocales } from "@/i18n";
import { cn } from "@/lib/utils";

/** ID/EN toggle. Offers only locales that both the server and the bundled catalogs support. */
export function LanguageSwitcher({ serverLocales }: { serverLocales: readonly string[] }) {
  const { t, i18n } = useTranslation();
  const locales = supportedLocales.filter((l: Locale) => serverLocales.filter(isLocale).includes(l));
  const active = i18n.resolvedLanguage;

  return (
    <div role="group" aria-label={t("language.label")} className="inline-flex rounded-md border p-0.5">
      {locales.map((lng) => (
        <button
          key={lng}
          type="button"
          lang={lng}
          aria-pressed={active === lng}
          aria-label={t("language.switchTo", { language: t(`language.${lng}`) })}
          onClick={() => void setLocale(lng)}
          className={cn(
            "rounded px-2.5 py-1 text-sm font-medium transition-colors",
            active === lng ? "bg-primary text-primary-foreground" : "hover:bg-accent",
          )}
        >
          {t(`language.short.${lng}`)}
        </button>
      ))}
    </div>
  );
}
