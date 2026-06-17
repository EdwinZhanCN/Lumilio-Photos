import { useLayoutEffect, type ReactNode } from "react";
import { usePreferencesStore } from "@/features/settings/preferences";
import { applyThemePreferencesToDocument } from "./daisyuiThemes";
import { useSystemThemeMode } from "./useSystemThemeMode";

/** Applies persisted theme preferences to `<html data-theme="...">`. */
export function ThemeEffects({ children }: { children: ReactNode }) {
  const theme = usePreferencesStore((s) => s.theme);
  const systemThemeMode = useSystemThemeMode();

  useLayoutEffect(() => {
    applyThemePreferencesToDocument(theme, systemThemeMode);
  }, [theme, systemThemeMode]);

  return children;
}
