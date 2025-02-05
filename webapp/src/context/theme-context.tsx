import React, { createContext, useContext, useReducer, ReactNode } from "react";

const ThemeProps = {
  key: "theme",
  light: "light",
  dark: "dark",
} as const;

type Theme = typeof ThemeProps.light | typeof ThemeProps.dark;

type ThemeState = { theme: Theme };

type ThemeAction =
  | { type: "SET_LIGHT" }
  | { type: "SET_DARK" }
  | { type: "TOGGLE" };

const themeReducer = (state: ThemeState, action: ThemeAction): ThemeState => {
  switch (action.type) {
    case "SET_LIGHT":
      return { theme: ThemeProps.light };
    case "SET_DARK":
      return { theme: ThemeProps.dark };
    case "TOGGLE":
      return {
        theme:
          state.theme === ThemeProps.dark ? ThemeProps.light : ThemeProps.dark,
      };
    default:
      return state;
  }
};

const ThemeContext = createContext<
  | {
      state: ThemeState;
      dispatch: React.Dispatch<ThemeAction>;
    }
  | undefined
>(undefined);

interface ThemeProviderProps {
  children: ReactNode;
}

export function ThemeProvider({ children }: ThemeProviderProps) {
  const [state, dispatch] = useReducer(themeReducer, {
    theme: (localStorage.getItem(ThemeProps.key) as Theme) || ThemeProps.light,
  });

  React.useEffect(() => {
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

export const useTheme = () => {
  const context = useContext(ThemeContext);

  if (!context) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }

  return context;
};
