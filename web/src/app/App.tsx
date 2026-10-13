import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { BrowserRouter, Route, Routes } from "react-router";

import { BootstrapGate } from "@/features/bootstrap/BootstrapGate";
import { PlaceholderPage } from "@/features/placeholder/PlaceholderPage";

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
  });
}

export function App({ queryClient }: { queryClient?: QueryClient }) {
  const [client] = useState(() => queryClient ?? createQueryClient());
  return (
    <QueryClientProvider client={client}>
      <BrowserRouter>
        <BootstrapGate>
          <Routes>
            {/* Every path renders the placeholder until the shell routes arrive (v0.2.2). */}
            <Route path="*" element={<PlaceholderPage />} />
          </Routes>
        </BootstrapGate>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
