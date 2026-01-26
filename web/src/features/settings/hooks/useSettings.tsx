import { useContext } from "react";
import { SettingsContext } from "../SettingsProvider";
import { SettingsContextValue } from "../settings.type.ts";

export const useSettingsContext = (): SettingsContextValue => {
  const context = useContext(SettingsContext);
  if (context === undefined) {
    throw new Error("useSettings must be used within a SettingsProvider");
  }
  return context;
};
