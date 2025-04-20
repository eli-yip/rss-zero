import { HeroUIProvider, ToastProvider } from "@heroui/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
// import { ReactQueryDevtools } from "@tanstack/react-query-devtools";
import type { NavigateOptions } from "react-router-dom";
import { useHref, useNavigate } from "react-router-dom";

import { ThemeProvider } from "@/context/theme-context-provider";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 1000 * 60 * 5,
    },
  },
});

declare module "@react-types/shared" {
  interface RouterConfig {
    routerOptions: NavigateOptions;
  }
}

export function Provider({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();

  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <HeroUIProvider navigate={navigate} useHref={useHref}>
          <ToastProvider />
          {children}
        </HeroUIProvider>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
