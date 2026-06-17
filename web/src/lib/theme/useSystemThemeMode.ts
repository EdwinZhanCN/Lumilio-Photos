import { useEffect, useState } from "react";
import { getSystemThemeMode, type ThemeMode } from "./daisyuiThemes";

export function useSystemThemeMode(): ThemeMode {
  const [systemThemeMode, setSystemThemeMode] =
    useState<ThemeMode>(getSystemThemeMode);

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

  return systemThemeMode;
}
