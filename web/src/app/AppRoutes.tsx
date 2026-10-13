import { lazy, Suspense } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, Route, Routes } from "react-router";

import { useBootstrap } from "@/features/bootstrap/regional";

import { NotFoundPage } from "./pages/StatusPages";
import { buildRoutes } from "./routes";
import { AppShell } from "./shell/AppShell";

// Developer-only page outside the shell and the registry (ISS-029); loaded on demand.
const DesignSystemPage = lazy(() =>
  import("@/features/design-system/DesignSystemPage").then((m) => ({ default: m.DesignSystemPage })),
);

function PageFallback() {
  const { t } = useTranslation();
  return (
    <p role="status" className="p-8 text-muted-foreground">
      {t("app.loading")}
    </p>
  );
}

export function AppRoutes() {
  const boot = useBootstrap();
  return (
    <Routes>
      <Route
        path="/design-system"
        element={
          <Suspense fallback={<PageFallback />}>
            <DesignSystemPage />
          </Suspense>
        }
      />
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/dashboard" replace />} />
        {buildRoutes(boot.menus).map((r) => (
          <Route key={r.menu.key} path={r.path} element={r.element} />
        ))}
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}
