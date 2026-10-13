import { ChevronRight, Menu } from "lucide-react";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useLocation } from "react-router";

import { LanguageSwitcher } from "@/components/domain/LanguageSwitcher";
import { ThemeSwitcher } from "@/components/domain/ThemeSwitcher";
import { Button } from "@/components/ui/button";
import { useBootstrap } from "@/features/bootstrap/regional";

import { Clock } from "./Clock";
import { findMenu, useMenuText } from "./menu";

/** Page title + breadcrumb, live clock, language & theme. No user menu until Auth (v0.3). */
export function Header({ onOpenMenu }: { onOpenMenu: () => void }) {
  const { t } = useTranslation(["shell", "common"]);
  const boot = useBootstrap();
  const { menuLabel, groupLabel } = useMenuText();
  const { pathname } = useLocation();
  const menu = findMenu(boot.menus, pathname);
  const title = menu ? menuLabel(menu.key) : t("pages.notFound.title");
  const crumbs = [t("common:app.name"), ...(menu ? [groupLabel(menu.group)] : []), title];

  useEffect(() => {
    document.title = `${title} · ${t("common:app.name")}`;
  }, [title, t]);

  return (
    <header className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b bg-background px-4 py-3 lg:px-6">
      <Button variant="ghost" size="icon" className="lg:hidden" onClick={onOpenMenu} aria-label={t("layout.openMenu")}>
        <Menu aria-hidden />
      </Button>
      <div className="min-w-0 flex-1">
        <nav aria-label={t("layout.breadcrumb")}>
          <ol className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
            {crumbs.map((c, i) => (
              <li key={`${i}-${c}`} className="flex items-center gap-1">
                {i > 0 && <ChevronRight className="size-3" aria-hidden />}
                <span aria-current={i === crumbs.length - 1 ? "page" : undefined}>{c}</span>
              </li>
            ))}
          </ol>
        </nav>
        <h1 className="truncate text-lg font-semibold tracking-tight">{title}</h1>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Clock />
        <LanguageSwitcher serverLocales={boot.supported_locales} />
        <ThemeSwitcher />
      </div>
    </header>
  );
}
