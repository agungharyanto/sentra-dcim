import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { BrowserRouter, Route, Routes } from "react-router";

import { ThemeProvider } from "@/components/domain/ThemeProvider";
import { Toaster } from "@/components/ui/toast";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AuthProvider } from "@/features/auth/AuthProvider";
import { ForcedPasswordPage } from "@/features/auth/ForcedPasswordPage";
import { LoginPage } from "@/features/auth/LoginPage";
import { RequireAuth } from "@/features/auth/RequireAuth";
import { BootstrapGate } from "@/features/bootstrap/BootstrapGate";
import { ApiError } from "@/lib/api";

import { AppRoutes } from "./AppRoutes";

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // One retry for network/server errors; a 4xx (not found, invalid locale, no session) will not change on retry.
        retry: (failures, error) =>
          failures < 1 && !(error instanceof ApiError && error.status >= 400 && error.status < 500),
        refetchOnWindowFocus: false,
      },
    },
  });
}

export function App({ queryClient }: { queryClient?: QueryClient }) {
  const [client] = useState(() => queryClient ?? createQueryClient());
  return (
    <QueryClientProvider client={client}>
      <ThemeProvider>
        <TooltipProvider>
          <BrowserRouter>
            <AuthProvider>
              <Routes>
                {/* Login and the forced password change render without the bootstrap (it needs a session). */}
                <Route path="/login" element={<LoginPage />} />
                <Route
                  path="/change-password"
                  element={
                    <RequireAuth passwordChange>
                      <ForcedPasswordPage />
                    </RequireAuth>
                  }
                />
                <Route
                  path="*"
                  element={
                    <RequireAuth>
                      <BootstrapGate>
                        <AppRoutes />
                      </BootstrapGate>
                    </RequireAuth>
                  }
                />
              </Routes>
            </AuthProvider>
          </BrowserRouter>
          <Toaster />
        </TooltipProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
