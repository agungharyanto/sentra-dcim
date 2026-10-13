import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { BrowserRouter } from "react-router";

import { ThemeProvider } from "@/components/domain/ThemeProvider";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { BootstrapGate } from "@/features/bootstrap/BootstrapGate";
import { ApiError } from "@/lib/api";

import { AppRoutes } from "./AppRoutes";

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // One retry for network/server errors; a 4xx (not found, invalid locale) will not change on retry.
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
            <BootstrapGate>
              <AppRoutes />
            </BootstrapGate>
          </BrowserRouter>
          <Toaster />
        </TooltipProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
