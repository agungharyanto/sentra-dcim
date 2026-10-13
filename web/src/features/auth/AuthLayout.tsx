import { type ReactNode, useEffect } from "react";
import { useTranslation } from "react-i18next";

import { LanguageSwitcher } from "@/components/domain/LanguageSwitcher";
import { Logo } from "@/components/domain/Logo";
import { ThemeSwitcher } from "@/components/domain/ThemeSwitcher";
import { supportedLocales } from "@/i18n";

/** Centred card layout for the pages outside the shell (login, forced password change). */
export function AuthLayout({ title, children }: { title: string; children: ReactNode }) {
  const { t } = useTranslation();
  useEffect(() => {
    document.title = `${title} · ${t("app.name")}`;
  }, [title, t]);
  return (
    <div className="flex min-h-dvh flex-col bg-background text-foreground">
      <header className="flex flex-wrap items-center justify-between gap-2 px-4 py-3 sm:px-6">
        <Logo />
        <div className="flex items-center gap-2">
          <LanguageSwitcher serverLocales={supportedLocales} />
          <ThemeSwitcher />
        </div>
      </header>
      <main id="main-content" className="flex flex-1 items-start justify-center px-4 pt-[8vh] pb-12 sm:items-center sm:pt-0">
        <div className="w-full max-w-sm">{children}</div>
      </main>
      <footer className="px-4 pb-4 text-center text-xs text-muted-foreground">{t("app.tagline")}</footer>
    </div>
  );
}
