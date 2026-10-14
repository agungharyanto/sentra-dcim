import { useTranslation } from "react-i18next";
import { Navigate, NavLink, Route, Routes } from "react-router";

import { NotFoundPage } from "@/app/pages/StatusPages";
import { cn } from "@/lib/utils";

import { AllowedIpPage } from "./AllowedIpPage";
import { RegionalSettingsPage } from "./RegionalSettingsPage";

const sections = [
  { path: "regional", label: "nav.regional" },
  { path: "allowed-ip", label: "nav.allowedIp" },
] as const;

/** Settings (/settings, permission settings.manage): one sub-page per group — Regional (v0.3.5), Allowed IP (v0.3.6). */
export function SettingsModule() {
  const { t } = useTranslation("settings");
  return (
    <div className="max-w-4xl space-y-4">
      <nav aria-label={t("nav.label")} className="flex gap-1 border-b">
        {sections.map((s) => (
          <NavLink
            key={s.path}
            to={`/settings/${s.path}`}
            className={({ isActive }) =>
              cn(
                "-mb-px border-b-2 px-3 py-2 text-sm",
                isActive ? "border-primary font-medium text-foreground" : "border-transparent text-muted-foreground hover:text-foreground",
              )
            }
          >
            {t(s.label)}
          </NavLink>
        ))}
      </nav>
      <Routes>
        <Route index element={<Navigate to="regional" replace />} />
        <Route path="regional" element={<RegionalSettingsPage />} />
        <Route path="allowed-ip" element={<AllowedIpPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </div>
  );
}
