// src/features/settings/SettingsProvider.tsx
import { createContext, useReducer, ReactNode, useEffect } from "react";
import { SettingsContextValue, SettingsState } from "./types";
import { SettingsReducer, initialState } from "./settings.reducer";
import { getCurrentLanguage, changeLanguage } from "@/lib/i18n.tsx";

export const SettingsContext = createContext<SettingsContextValue | undefined>(
  undefined,
);

const STORAGE_KEY = "app_settings_v1";

function loadPersisted(): SettingsState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<SettingsState>;
      return {
        lumen: { ...initialState.lumen, ...parsed.lumen },
        server: { ...initialState.server, ...parsed.server },
        ui: {
          ...initialState.ui,
          ...parsed.ui,
          language: parsed.ui?.language ?? getCurrentLanguage(),
        },
      };
    }
  } catch (e) {
    console.warn("[SettingsProvider] Failed to parse stored settings", e);
  }
  return {
    ...initialState,
    ui: {
      ...initialState.ui,
      language: getCurrentLanguage(),
    },
  };
}

export const SettingsProvider = ({ children }: { children: ReactNode }) => {
  const [state, dispatch] = useReducer(
    SettingsReducer,
    undefined,
    loadPersisted,
  );

  // Persist settings to localStorage on any change
  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch (e) {
      console.warn("[SettingsProvider] Failed to persist settings", e);
    }
  }, [state]);

  // Sync <html lang="..."> attribute globally
  useEffect(() => {
    if (state.ui.language) {
      document.documentElement.setAttribute("lang", state.ui.language);
      changeLanguage(state.ui.language);
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
