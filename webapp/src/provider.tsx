import type { NavigateOptions } from "react-router-dom";

import { HeroUIProvider } from "@heroui/react";
import { useHref, useNavigate } from "react-router-dom";

import { ThemeProvider } from "@/context/theme-context-provider";

declare module "@react-types/shared" {
  interface RouterConfig {
    routerOptions: NavigateOptions;
  }
}

export function Provider({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate();

  return (
    <ThemeProvider>
      <HeroUIProvider navigate={navigate} useHref={useHref}>
        {children}
      </HeroUIProvider>
    </ThemeProvider>
  );
}
