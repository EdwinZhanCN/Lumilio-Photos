// src/features/settings/SettingsProvider.tsx
import { createContext, useReducer, ReactNode, useEffect } from "react";
import { SettingsContextValue } from "./settings.type.ts";
import { SettingsReducer, initialState } from "./settings.reducer";
import { changeLanguage, getCurrentLanguage } from "@/lib/i18n.tsx";
import {
  persistSettingsState,
  resolveInitialSettingsState,
} from "./settings.persistence";

export const SettingsContext = createContext<SettingsContextValue | undefined>(
  undefined,
);

export const SettingsProvider = ({ children }: { children: ReactNode }) => {
  const [state, dispatch] = useReducer(
    SettingsReducer,
    initialState,
    resolveInitialSettingsState,
  );

  // Persist settings to localStorage on any change
  useEffect(() => {
    persistSettingsState(state);
  }, [state]);

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
    state,
    dispatch,
  };

  return (
    <SettingsContext.Provider value={value}>
      {children}
    </SettingsContext.Provider>
  );
};
