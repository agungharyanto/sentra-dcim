import { PlugZap } from "lucide-react";
import { useTranslation } from "react-i18next";

import { StatePage } from "@/app/pages/StatePage";

/** Dashboard (v0.2.3): empty state only — no sample data until telemetry is connected (v0.6). */
export function DashboardPage() {
  const { t } = useTranslation("shell");
  return (
    <StatePage
      testId="dashboard-empty"
      icon={PlugZap}
      title={t("dashboard.emptyTitle")}
      body={t("dashboard.emptyBody")}
    />
  );
}
