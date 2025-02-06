import { useReducer, ReactNode, useEffect } from "react";

import { ThemeContext, ThemeProps, themeReducer, Theme } from "./theme-context";

interface ThemeProviderProps {
  children: ReactNode;
}

export function ThemeProvider({ children }: ThemeProviderProps) {
  const [state, dispatch] = useReducer(themeReducer, {
    theme: (localStorage.getItem(ThemeProps.key) as Theme) || ThemeProps.light,
  });

  useEffect(() => {
    localStorage.setItem(ThemeProps.key, state.theme);
    document.documentElement.classList.remove(
      ThemeProps.light,
      ThemeProps.dark,
    );
    document.documentElement.classList.add(state.theme);
  }, [state.theme]);

  return (
    <ThemeContext.Provider value={{ state, dispatch }}>
      {children}
    </ThemeContext.Provider>
  );
}
