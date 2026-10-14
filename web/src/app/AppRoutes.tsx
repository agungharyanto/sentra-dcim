import { lazy, Suspense } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, Route, Routes } from "react-router";

import { AccountPasswordPage } from "@/features/auth/AccountPasswordPage";
import { usePermissions } from "@/features/auth/permissions";
import { SecurityPage } from "@/features/auth/SecurityPage";
import { SessionsPage } from "@/features/auth/SessionsPage";
import { useBootstrap } from "@/features/bootstrap/regional";

import { ForbiddenPage, NotFoundPage } from "./pages/StatusPages";
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
  const perms = usePermissions();
  return (
    <Routes>
      {/* Developer showcase: Admin only (ISS-029); others get Access denied without loading the chunk. */}
      <Route
        path="/design-system"
        element={
          perms.has("ui.design_system") ? (
            <Suspense fallback={<PageFallback />}>
              <DesignSystemPage />
            </Suspense>
          ) : (
            <main className="p-8">
              <ForbiddenPage />
            </main>
          )
        }
      />
      <Route element={<AppShell />}>
        <Route index element={<Navigate to="/dashboard" replace />} />
        {buildRoutes(boot.menus, perms).map((r) => (
          <Route key={r.menu.key} path={r.path} element={r.element} />
        ))}
        {/* Own account pages (not registry menus; reached from the user menu). */}
        <Route path="/account/sessions" element={<SessionsPage />} />
        <Route path="/account/password" element={<AccountPasswordPage />} />
        <Route path="/account/security" element={<SecurityPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}
