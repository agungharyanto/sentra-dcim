import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { lazy, Suspense, useState } from "react";
import { useTranslation } from "react-i18next";
import { BrowserRouter, Route, Routes } from "react-router";

import { ThemeProvider } from "@/components/domain/ThemeProvider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { BootstrapGate } from "@/features/bootstrap/BootstrapGate";
import { PlaceholderPage } from "@/features/placeholder/PlaceholderPage";

// Developer-only page: loaded on demand so it never weighs on the main bundle.
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

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
  });
}

export function App({ queryClient }: { queryClient?: QueryClient }) {
  const [client] = useState(() => queryClient ?? createQueryClient());
  return (
    <QueryClientProvider client={client}>
      <ThemeProvider>
        <TooltipProvider>
          <BrowserRouter>
            <BootstrapGate>
              <Routes>
                {/* Internal developer showcase: deliberately not in the menu registry (ISS-029). */}
                <Route
                  path="/design-system"
                  element={
                    <Suspense fallback={<PageFallback />}>
                      <DesignSystemPage />
                    </Suspense>
                  }
                />
                {/* Every other path renders the placeholder until the shell routes arrive (v0.2.3). */}
                <Route path="*" element={<PlaceholderPage />} />
              </Routes>
            </BootstrapGate>
          </BrowserRouter>
          <Toaster />
        </TooltipProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
