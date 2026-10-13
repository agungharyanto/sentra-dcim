import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Outlet, useLocation } from "react-router";

import { Logo } from "@/components/domain/Logo";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { useBootstrap } from "@/features/bootstrap/regional";
import { cn } from "@/lib/utils";

import { ErrorPage } from "../pages/StatusPages";
import { ErrorBoundary } from "./ErrorBoundary";
import { Header } from "./Header";
import { readSidebarCollapsed, writeSidebarCollapsed } from "./sidebar-state";
import { SidebarNav } from "./SidebarNav";

/** Application layout: skip link, sidebar (≥ lg) or drawer (< lg), header, and the routed page in <main>. */
export function AppShell() {
  const { t } = useTranslation(["shell", "common"]);
  const boot = useBootstrap();
  const { pathname } = useLocation();
  const [collapsed, setCollapsed] = useState(readSidebarCollapsed);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const toggle = () => {
    writeSidebarCollapsed(!collapsed);
    setCollapsed(!collapsed);
  };

  return (
    <div className="flex min-h-dvh bg-background text-foreground">
      <a
        href="#main-content"
        className="sr-only z-50 rounded-md bg-primary px-4 py-2 text-primary-foreground focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
      >
        {t("layout.skipToContent")}
      </a>

      <aside
        id="app-sidebar"
        data-collapsed={collapsed}
        className={cn(
          "sticky top-0 hidden h-dvh shrink-0 flex-col border-r bg-card lg:flex",
          collapsed ? "w-16" : "w-72",
        )}
      >
        <div className={cn("flex h-14 items-center border-b px-4", collapsed && "justify-center px-2")}>
          <Logo withName={!collapsed} />
        </div>
        <nav aria-label={t("layout.mainNav")} className="flex-1 overflow-y-auto p-2">
          <SidebarNav collapsed={collapsed} />
        </nav>
        <div className={cn("flex items-center gap-2 border-t p-2", collapsed ? "flex-col" : "justify-between")}>
          <span className={cn("px-2 font-mono text-xs text-muted-foreground", collapsed && "sr-only")} data-testid="app-version">
            {t("layout.version", { version: boot.version })}
          </span>
          <Button
            variant="ghost"
            size="icon"
            onClick={toggle}
            aria-controls="app-sidebar"
            aria-expanded={!collapsed}
            aria-label={collapsed ? t("layout.expand") : t("layout.collapse")}
            title={collapsed ? t("layout.expand") : t("layout.collapse")}
          >
            {collapsed ? <PanelLeftOpen aria-hidden /> : <PanelLeftClose aria-hidden />}
          </Button>
        </div>
      </aside>

      <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
        <SheetContent side="left" className="w-72 overflow-y-auto lg:hidden" data-testid="nav-drawer">
          <SheetHeader className="border-b">
            <SheetTitle>
              <Logo />
            </SheetTitle>
            <SheetDescription className="sr-only">{t("layout.drawerDescription")}</SheetDescription>
          </SheetHeader>
          <nav aria-label={t("layout.mainNav")} className="px-2">
            <SidebarNav onNavigate={() => setDrawerOpen(false)} />
          </nav>
          <p className="mt-auto border-t p-4 font-mono text-xs text-muted-foreground">
            {t("layout.version", { version: boot.version })}
          </p>
        </SheetContent>
      </Sheet>

      <div className="flex min-w-0 flex-1 flex-col">
        <Header onOpenMenu={() => setDrawerOpen(true)} />
        <main id="main-content" tabIndex={-1} className="flex-1 p-4 outline-none lg:p-6">
          {/* key: a new page gets a fresh boundary, so an error on one page is cleared by navigating away. */}
          <ErrorBoundary key={pathname} fallback={<ErrorPage />}>
            <Outlet />
          </ErrorBoundary>
        </main>
      </div>
    </div>
  );
}
