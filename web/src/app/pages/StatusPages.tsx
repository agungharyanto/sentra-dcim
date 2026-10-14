import { Clock, FileQuestion, ShieldX, TriangleAlert, Unplug } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router";

import { Button } from "@/components/ui/button";
import type { MenuItem } from "@/features/bootstrap/api";

import { useMenuText } from "../shell/menu";
import { StatePage } from "./StatePage";

function BackToDashboard({ label }: { label: string }) {
  return (
    <Button asChild variant="outline">
      <Link to="/dashboard">{label}</Link>
    </Button>
  );
}

/** Standard page for registry menus with status coming_soon. */
export function ComingSoonPage({ menu }: { menu: MenuItem }) {
  const { t } = useTranslation("shell");
  const { menuLabel, menuDescription } = useMenuText();
  const name = menuLabel(menu.key);
  return (
    <StatePage
      testId="coming-soon"
      icon={Clock}
      title={t("pages.comingSoon.title")}
      body={`${menuDescription(menu.key)} ${t("pages.comingSoon.body", { menu: name })}`}
      action={<BackToDashboard label={t("pages.comingSoon.back")} />}
    />
  );
}

/** Standard page for registry menus with status not_connected. */
export function NotConnectedPage({ menu }: { menu: MenuItem }) {
  const { t } = useTranslation("shell");
  const { menuLabel } = useMenuText();
  return (
    <StatePage
      testId="not-connected"
      icon={Unplug}
      title={t("pages.notConnected.title")}
      body={t("pages.notConnected.body", { menu: menuLabel(menu.key) })}
      action={<BackToDashboard label={t("pages.notConnected.back")} />}
    />
  );
}

/** A registry menu (or one of its pages) the signed-in user has no permission for. */
export function ForbiddenPage() {
  const { t } = useTranslation("shell");
  return (
    <StatePage
      testId="forbidden"
      icon={ShieldX}
      title={t("pages.forbidden.title")}
      body={t("pages.forbidden.body")}
      action={<BackToDashboard label={t("pages.forbidden.back")} />}
    />
  );
}

export function NotFoundPage() {
  const { t } = useTranslation("shell");
  const { pathname } = useLocation();
  return (
    <StatePage
      testId="not-found"
      icon={FileQuestion}
      title={t("pages.notFound.title")}
      body={t("pages.notFound.body", { path: pathname })}
      action={<BackToDashboard label={t("pages.notFound.back")} />}
    />
  );
}

export function ErrorPage() {
  const { t } = useTranslation("shell");
  return (
    <StatePage
      testId="page-error"
      icon={TriangleAlert}
      title={t("pages.error.title")}
      body={t("pages.error.body")}
      action={
        <Button variant="outline" onClick={() => window.location.reload()}>
          {t("pages.error.reload")}
        </Button>
      }
    />
  );
}
