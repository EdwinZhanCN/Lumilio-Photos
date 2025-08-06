// src/features/settings/SettingsProvider.tsx
import { createContext, useReducer, ReactNode } from "react";
import { SettingsContextValue } from "./types";
import { SettingsReducer, initialState } from "./settings.reducer";

export const SettingsContext = createContext<SettingsContextValue | undefined>(
  undefined,
);

export const SettingsProvider = ({ children }: { children: ReactNode }) => {
  const [state, dispatch] = useReducer(SettingsReducer, initialState);

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
