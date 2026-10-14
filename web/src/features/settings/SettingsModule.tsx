import { useTranslation } from "react-i18next";
import { Navigate, NavLink, Route, Routes } from "react-router";

import { NotFoundPage } from "@/app/pages/StatusPages";
import { cn } from "@/lib/utils";

import { RegionalSettingsPage } from "./RegionalSettingsPage";

/** Settings (/settings, permission settings.manage): one sub-page per group; Regional since v0.3.5. */
export function SettingsModule() {
  const { t } = useTranslation("settings");
  return (
    <div className="max-w-4xl space-y-4">
      <nav aria-label={t("nav.label")} className="flex gap-1 border-b">
        <NavLink
          to="/settings/regional"
          className={({ isActive }) =>
            cn(
              "-mb-px border-b-2 px-3 py-2 text-sm",
              isActive ? "border-primary font-medium text-foreground" : "border-transparent text-muted-foreground hover:text-foreground",
            )
          }
        >
          {t("nav.regional")}
        </NavLink>
      </nav>
      <Routes>
        <Route index element={<Navigate to="regional" replace />} />
        <Route path="regional" element={<RegionalSettingsPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </div>
  );
}
