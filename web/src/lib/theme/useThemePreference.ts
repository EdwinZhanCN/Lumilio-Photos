import { usePreference } from "@/features/settings/preferences";
import type { ThemePreferences } from "./daisyuiThemes";

export function useThemePreference(): [
  ThemePreferences,
  (value: ThemePreferences) => void,
] {
  return usePreference("theme");
}
