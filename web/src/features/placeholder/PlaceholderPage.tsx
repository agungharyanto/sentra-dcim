import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import { useBootstrap, useFormatters } from "@/features/bootstrap/regional";
import type { resources } from "@/i18n/resources";

type MenuKey = keyof (typeof resources)["id"]["shell"]["sidebar"];

const clockFormat: Intl.DateTimeFormatOptions = { dateStyle: "full", timeStyle: "long" };

/** Temporary landing page (v0.2.1) until the UI shell arrives in v0.2.2+. */
export function PlaceholderPage() {
  const { t } = useTranslation(["shell", "common"]);
  const boot = useBootstrap();
  const fmt = useFormatters();
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const id = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(id);
  }, []);

  useEffect(() => {
    document.title = t("common:app.name");
  }, [t]);

  return (
    <div className="min-h-dvh bg-background text-foreground">
      <header className="flex items-center justify-between border-b px-6 py-4">
        <div>
          <p className="text-lg font-semibold">{t("common:app.name")}</p>
          <p className="text-sm text-muted-foreground">{t("common:app.tagline")}</p>
        </div>
        <LanguageSwitcher serverLocales={boot.supported_locales} />
      </header>

      <main className="mx-auto max-w-3xl space-y-6 p-6">
        <section className="space-y-2">
          <h1 className="text-2xl font-semibold tracking-tight">{t("placeholder.title")}</h1>
          <p className="text-muted-foreground">{t("placeholder.subtitle")}</p>
        </section>

        <dl className="grid gap-4 rounded-lg border p-4 sm:grid-cols-2">
          <div>
            <dt className="text-sm text-muted-foreground">{t("placeholder.version")}</dt>
            <dd className="font-mono" data-testid="version">
              {boot.version}
            </dd>
          </div>
          <div>
            <dt className="text-sm text-muted-foreground">{t("placeholder.locale")}</dt>
            <dd data-testid="locale">{t(`common:language.${fmt.locale === "en" ? "en" : "id"}`)}</dd>
          </div>
          <div>
            <dt className="text-sm text-muted-foreground">{t("placeholder.timezone")}</dt>
            <dd className="font-mono" data-testid="timezone">
              {fmt.timezone}
            </dd>
          </div>
          <div>
            <dt className="text-sm text-muted-foreground">{t("placeholder.now")}</dt>
            <dd className="font-mono" data-testid="now">
              {fmt.dateTime(now, clockFormat)}
            </dd>
          </div>
        </dl>

        <section className="space-y-2">
          <h2 className="font-medium">{t("placeholder.menus", { count: boot.menus.length })}</h2>
          <ul className="divide-y rounded-lg border">
            {boot.menus.map((m) => (
              <li key={m.key} className="flex items-center justify-between px-4 py-2 text-sm">
                <span>{t(`sidebar.${m.key as MenuKey}`)}</span>
                <span className="text-muted-foreground">{t(`menuStatus.${m.status}`)}</span>
              </li>
            ))}
          </ul>
        </section>
      </main>
    </div>
  );
}
