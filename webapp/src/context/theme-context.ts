import type React from "react";
import { createContext, useContext } from "react";

export const ThemeProps = {
  key: "theme",
  light: "light",
  dark: "dark",
} as const;

export type Theme = typeof ThemeProps.light | typeof ThemeProps.dark;

type ThemeState = { theme: Theme };

type ThemeAction =
  | { type: "SET_LIGHT" }
  | { type: "SET_DARK" }
  | { type: "TOGGLE" };

export const themeReducer = (
  state: ThemeState,
  action: ThemeAction,
): ThemeState => {
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

export const ThemeContext = createContext<
  | {
      state: ThemeState;
      dispatch: React.Dispatch<ThemeAction>;
    }
  | undefined
>(undefined);

export const useTheme = () => {
  const context = useContext(ThemeContext);

  if (!context) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }

  return context;
};
