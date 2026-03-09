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
  upload?: {
    max_total_files: number; // 总文件上传数量限制
    low_power_mode?: boolean; // 低功耗模式开关
    chunk_size_mb?: number; // 客户端分片大小（MB）
    max_concurrent_chunks?: number; // 分片并发上限
    use_server_config?: boolean; // 是否采用后端返回的上传配置
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
  // Upload Actions
  | { type: "SET_UPLOAD_MAX_TOTAL_FILES"; payload: number }
  | { type: "SET_UPLOAD_LOW_POWER_MODE"; payload: boolean }
  | { type: "SET_UPLOAD_CHUNK_SIZE_MB"; payload: number }
  | { type: "SET_UPLOAD_MAX_CONCURRENT_CHUNKS"; payload: number }
  | { type: "SET_UPLOAD_USE_SERVER_CONFIG"; payload: boolean }
  // Server Actions
  | { type: "SET_SERVER_UPDATE_TIMESPAN"; payload: number };

export interface SettingsContextValue {
  resolvedThemeMode: ThemeMode;
  state: SettingsState;
  dispatch: React.Dispatch<SettingsAction>;
}
