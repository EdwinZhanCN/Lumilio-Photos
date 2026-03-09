// src/features/settings/SettingsProvider.tsx
import {
  createContext,
  useReducer,
  ReactNode,
  useEffect,
  useLayoutEffect,
  useState,
} from "react";
import { SettingsContextValue } from "./settings.type.ts";
import { SettingsReducer, initialState } from "./settings.reducer";
import { changeLanguage, getCurrentLanguage } from "@/lib/i18n.tsx";
import {
  persistSettingsState,
  resolveInitialSettingsState,
} from "./settings.persistence";
import {
  applyThemePreferencesToDocument,
  getSystemThemeMode,
  resolveActiveThemeMode,
} from "@/lib/theme/daisyuiThemes";

export const SettingsContext = createContext<SettingsContextValue | undefined>(
  undefined,
);

export const SettingsProvider = ({ children }: { children: ReactNode }) => {
  const [systemThemeMode, setSystemThemeMode] = useState(getSystemThemeMode);
  const [state, dispatch] = useReducer(
    SettingsReducer,
    initialState,
    resolveInitialSettingsState,
  );

  useEffect(() => {
    if (
      typeof window === "undefined" ||
      typeof window.matchMedia !== "function"
    ) {
      return;
    }

    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    const syncSystemTheme = () => {
      setSystemThemeMode(mediaQuery.matches ? "dark" : "light");
    };

    syncSystemTheme();
    mediaQuery.addEventListener("change", syncSystemTheme);

    return () => {
      mediaQuery.removeEventListener("change", syncSystemTheme);
    };
  }, []);

  // Persist settings to localStorage on any change
  useEffect(() => {
    persistSettingsState(state);
  }, [state]);

  // Sync concrete daisyUI theme onto <html data-theme="...">.
  useLayoutEffect(() => {
    applyThemePreferencesToDocument(state.ui.theme, systemThemeMode);
  }, [state.ui.theme, systemThemeMode]);

  // Sync <html lang="..."> attribute globally
  useEffect(() => {
    const nextLanguage = state.ui.language;
    if (!nextLanguage) return;

    if (document.documentElement.lang !== nextLanguage) {
      document.documentElement.setAttribute("lang", nextLanguage);
    }

    if (getCurrentLanguage() !== nextLanguage) {
      void changeLanguage(nextLanguage);
    }
  }, [state.ui.language]);

  const value: SettingsContextValue = {
    resolvedThemeMode: resolveActiveThemeMode(state.ui.theme, systemThemeMode),
    state,
    dispatch,
  };

  return (
    <SettingsContext.Provider value={value}>
      {children}
    </SettingsContext.Provider>
  );
};
