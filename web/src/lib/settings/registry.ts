export const SETTINGS_STORAGE_KEY = "app_settings";
export const LEGACY_SETTINGS_STORAGE_KEY = "app_settings_v1";
export const SETTINGS_STORAGE_VERSION = 5 as const;

export const THEME_STORAGE_KEY = "lumilio.settings.theme";
export const THEME_STORAGE_VERSION = 1 as const;
export const LEGACY_THEME_STORAGE_KEY = "theme";

export const PERFORMANCE_PREFERENCES_STORAGE_KEY =
  "lumilio.settings.performance_preferences";
export const PERFORMANCE_PREFERENCES_STORAGE_VERSION = 2 as const;
export const LEGACY_PERFORMANCE_PREFERENCES_STORAGE_KEY =
  "lumilio_performance_preferences";

export const ASSETS_STATE_STORAGE_KEY = "lumilio.settings.assets_state";
export const ASSETS_STATE_STORAGE_VERSION = 2 as const;
export const LEGACY_ASSETS_STATE_STORAGE_KEY = "assets_state_v1";

export type SettingsTruthSource =
  | "web_local_preference"
  | "server_runtime_capability";

export interface SettingRegistryEntry {
  path: string;
  truthSource: SettingsTruthSource;
  description: string;
  precedence: readonly string[];
}

// Registry for fields managed by SettingsProvider.
// This is the canonical ownership map for local settings state.
export const SETTINGS_REGISTRY: readonly SettingRegistryEntry[] = [
  {
    path: "ui.language",
    truthSource: "web_local_preference",
    description: "UI language preference used by i18n",
    precedence: ["user local setting", "browser language fallback", "en"],
  },
  {
    path: "ui.region",
    truthSource: "web_local_preference",
    description: "Region preference for map provider behavior",
    precedence: ["user local setting", "other"],
  },
  {
    path: "ui.theme.mode",
    truthSource: "web_local_preference",
    description: "Navbar appearance mode preference",
    precedence: ["user local setting", "light"],
  },
  {
    path: "ui.theme.followSystem",
    truthSource: "web_local_preference",
    description: "Whether theme mode follows the operating system preference",
    precedence: ["user local setting", "true"],
  },
  {
    path: "ui.theme.themes.light",
    truthSource: "web_local_preference",
    description: "Concrete daisyUI theme used while light mode is active",
    precedence: ["user local setting", "light"],
  },
  {
    path: "ui.theme.themes.dark",
    truthSource: "web_local_preference",
    description: "Concrete daisyUI theme used while dark mode is active",
    precedence: ["user local setting", "night"],
  },
  {
    path: "ui.working_repository_id",
    truthSource: "web_local_preference",
    description: "Current working repository scope for repository-aware views",
    precedence: ["user local setting", "all repositories"],
  },
  {
    path: "ui.asset_page.layout",
    truthSource: "web_local_preference",
    description: "Asset page layout preference",
    precedence: ["user local setting", "full"],
  },
  {
    path: "ui.asset_page.columns",
    truthSource: "web_local_preference",
    description: "Square asset page layout column count",
    precedence: ["user local setting", "6"],
  },
  {
    path: "ui.upload.max_total_files",
    truthSource: "web_local_preference",
    description: "Client-side upload file count guardrail",
    precedence: ["user local setting", "100"],
  },
  {
    path: "ui.upload.low_power_mode",
    truthSource: "web_local_preference",
    description: "Client-side low power upload mode toggle",
    precedence: ["user local setting", "true"],
  },
  {
    path: "ui.upload.chunk_size_mb",
    truthSource: "server_runtime_capability",
    description: "Chunk size override used when server config is disabled",
    precedence: [
      "server runtime config when use_server_config=true",
      "user local override",
      "adaptive default",
    ],
  },
  {
    path: "ui.upload.max_concurrent_chunks",
    truthSource: "server_runtime_capability",
    description:
      "Chunk concurrency override used when server config is disabled",
    precedence: [
      "server runtime config when use_server_config=true",
      "user local override",
      "adaptive default",
    ],
  },
  {
    path: "ui.upload.use_server_config",
    truthSource: "web_local_preference",
    description: "Switch for preferring server runtime upload config",
    precedence: ["user local setting", "true"],
  },
  {
    path: "server.update_timespan",
    truthSource: "web_local_preference",
    description: "Health poll interval in seconds",
    precedence: ["user local setting", "5"],
  },
] as const;

export type LocalSettingsOwner =
  | "settings_provider"
  | "performance_preferences"
  | "assets_provider";

export interface LocalStorageRegistryEntry {
  key: string;
  version: number;
  owner: LocalSettingsOwner;
  legacyKeys: readonly string[];
  description: string;
}

// Key-level registry for browser persistence ownership.
export const LOCAL_STORAGE_REGISTRY: readonly LocalStorageRegistryEntry[] = [
  {
    key: SETTINGS_STORAGE_KEY,
    version: SETTINGS_STORAGE_VERSION,
    owner: "settings_provider",
    legacyKeys: [LEGACY_SETTINGS_STORAGE_KEY],
    description: "App-level settings state",
  },
  {
    key: PERFORMANCE_PREFERENCES_STORAGE_KEY,
    version: PERFORMANCE_PREFERENCES_STORAGE_VERSION,
    owner: "performance_preferences",
    legacyKeys: [LEGACY_PERFORMANCE_PREFERENCES_STORAGE_KEY],
    description: "Performance preference profile and knobs",
  },
  {
    key: ASSETS_STATE_STORAGE_KEY,
    version: ASSETS_STATE_STORAGE_VERSION,
    owner: "assets_provider",
    legacyKeys: [LEGACY_ASSETS_STATE_STORAGE_KEY],
    description: "Persisted assets feature filters and selection state",
  },
] as const;
