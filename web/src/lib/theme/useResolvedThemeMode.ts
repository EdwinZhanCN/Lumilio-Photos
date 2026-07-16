import { usePreferencesStore } from "@/lib/preferences/preferences";
import { resolveActiveThemeMode, type ThemeMode } from "./daisyuiThemes";
import { useSystemThemeMode } from "./useSystemThemeMode";

export function useResolvedThemeMode(): ThemeMode {
  const theme = usePreferencesStore((s) => s.theme);
  const systemThemeMode = useSystemThemeMode();
  return resolveActiveThemeMode(theme, systemThemeMode);
}
