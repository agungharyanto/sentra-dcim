import { ChevronRight, CircleHelp, Menu } from "lucide-react";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router";

import { LanguageSwitcher } from "@/components/domain/LanguageSwitcher";
import { ThemeSwitcher } from "@/components/domain/ThemeSwitcher";
import { Button } from "@/components/ui/button";
import { accountPageTitleKey } from "@/features/auth/account";
import { UserMenu } from "@/features/auth/UserMenu";
import { useBootstrap } from "@/features/bootstrap/regional";
import { helpPathForMenu, useHelpIndex } from "@/features/help/api";

import { Clock } from "./Clock";
import { findMenu, useMenuText } from "./menu";

/** Page title + breadcrumb, live clock, help, language & theme, signed-in user menu. */
export function Header({ onOpenMenu }: { onOpenMenu: () => void }) {
  const { t } = useTranslation(["shell", "common", "help", "auth"]);
  const boot = useBootstrap();
  const { menuLabel, groupLabel } = useMenuText();
  const { pathname } = useLocation();
  const menu = findMenu(boot.menus, pathname);
  const helpIndex = useHelpIndex();
  const helpPath = helpPathForMenu(helpIndex.data?.pages, menu?.key);
  const account = accountPageTitleKey(pathname);
  const title = menu ? menuLabel(menu.key) : account ? t(`auth:${account}`) : t("pages.notFound.title");
  const group = menu ? groupLabel(menu.group) : account ? t("auth:account.group") : undefined;
  const crumbs = [t("common:app.name"), ...(group ? [group] : []), title];

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
        <Button asChild variant="outline" size="icon" title={t("help:openHelp")}>
          <Link to={helpPath} aria-label={t("help:openHelp")} data-testid="help-button">
            <CircleHelp aria-hidden />
          </Link>
        </Button>
        <LanguageSwitcher serverLocales={boot.supported_locales} />
        <ThemeSwitcher />
        <UserMenu />
      </div>
    </header>
  );
}
