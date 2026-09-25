"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { AuthProvider } from "@/contexts/AuthContext";
import { LocaleProvider } from "@/contexts/LocaleContext";
import { AppearanceProvider } from "@/contexts/AppearanceContext";
import { ThemeProvider } from "@/contexts/ThemeContext";
import { ConfirmProvider } from "@/contexts/ConfirmContext";
import { FlagProvider } from "@/contexts/FlagContext";

export function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 30 * 1000,
            refetchOnWindowFocus: true,
          },
        },
      })
  );

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AppearanceProvider>
          <LocaleProvider>
            {/* Dialog and notices render once, above every page. */}
            <FlagProvider>
              <ConfirmProvider>
                <AuthProvider>{children}</AuthProvider>
              </ConfirmProvider>
            </FlagProvider>
          </LocaleProvider>
        </AppearanceProvider>
      </ThemeProvider>
    </QueryClientProvider>
  );
}
