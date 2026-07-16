import { usePreference } from "@/lib/preferences/preferences";
import type { ThemePreferences } from "./daisyuiThemes";

export function useThemePreference(): [ThemePreferences, (value: ThemePreferences) => void] {
  return usePreference("theme");
}
