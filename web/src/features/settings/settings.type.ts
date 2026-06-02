import type {
  DaisyUIDarkThemeName,
  DaisyUILightThemeName,
  ThemeMode,
  ThemePreferences,
} from "@/lib/theme/daisyuiThemes";

export interface UIThemeSettings extends ThemePreferences {}

export interface UISettings {
  language?: "en" | "zh";
  region?: "china" | "other";
  working_repository_id?: string;
  theme: UIThemeSettings;
  asset_page?: {
    layout: "compact" | "full";
    columns: number;
  };
}

export interface ServerSettings {
  update_timespan: number;
}

export interface SettingsState {
  ui: UISettings;
  server: ServerSettings;
}

export type SettingsAction =
  // UI Actions
  | { type: "SET_THEME_FOLLOW_SYSTEM"; payload: boolean }
  | { type: "SET_THEME_MODE"; payload: ThemeMode }
  | { type: "SET_LIGHT_MODE_THEME"; payload: DaisyUILightThemeName }
  | { type: "SET_DARK_MODE_THEME"; payload: DaisyUIDarkThemeName }
  | { type: "SET_ASSETS_LAYOUT"; payload: "compact" | "full" }
  | { type: "SET_ASSETS_COLUMNS"; payload: number }
  | { type: "SET_LANGUAGE"; payload: "en" | "zh" }
  | { type: "SET_REGION"; payload: "china" | "other" }
  | { type: "SET_WORKING_REPOSITORY_ID"; payload: string | null }
  // Server Actions
  | { type: "SET_SERVER_UPDATE_TIMESPAN"; payload: number };

export interface SettingsContextValue {
  resolvedThemeMode: ThemeMode;
  state: SettingsState;
  dispatch: React.Dispatch<SettingsAction>;
}
