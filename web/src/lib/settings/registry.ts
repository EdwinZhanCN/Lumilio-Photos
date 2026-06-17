export const PREFERENCES_STORAGE_KEY = "lumilio.preferences";
export const PREFERENCES_STORAGE_VERSION = 2 as const;

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

export const SETTINGS_REGISTRY: readonly SettingRegistryEntry[] = [
  {
    path: "language",
    truthSource: "web_local_preference",
    description: "UI language preference used by i18n",
    precedence: ["user local setting", "browser language fallback", "en"],
  },
  {
    path: "region",
    truthSource: "web_local_preference",
    description: "Region preference for map provider behavior",
    precedence: ["user local setting", "other"],
  },
  {
    path: "theme.mode",
    truthSource: "web_local_preference",
    description: "Navbar appearance mode preference",
    precedence: ["user local setting", "light"],
  },
  {
    path: "theme.followSystem",
    truthSource: "web_local_preference",
    description: "Whether theme mode follows the operating system preference",
    precedence: ["user local setting", "true"],
  },
  {
    path: "theme.themes.light",
    truthSource: "web_local_preference",
    description: "Concrete daisyUI theme used while light mode is active",
    precedence: ["user local setting", "lumilio"],
  },
  {
    path: "theme.themes.dark",
    truthSource: "web_local_preference",
    description: "Concrete daisyUI theme used while dark mode is active",
    precedence: ["user local setting", "lumilio-dark"],
  },
  {
    path: "workingRepositoryId",
    truthSource: "web_local_preference",
    description: "Current working repository scope for repository-aware views",
    precedence: ["user local setting", "all repositories"],
  },
  {
    path: "assetPage.layout",
    truthSource: "web_local_preference",
    description: "Asset page layout preference",
    precedence: ["user local setting", "full"],
  },
  {
    path: "assetPage.columns",
    truthSource: "web_local_preference",
    description: "Square asset page layout column count",
    precedence: ["user local setting", "6"],
  },
  {
    path: "healthCheckIntervalMs",
    truthSource: "web_local_preference",
    description: "Health poll interval in milliseconds",
    precedence: ["user local setting", "30000"],
  },
] as const;

export type LocalSettingsOwner = "preferences_store" | "assets_provider";

export interface LocalStorageRegistryEntry {
  key: string;
  version: number;
  owner: LocalSettingsOwner;
  legacyKeys: readonly string[];
  description: string;
}

export const LOCAL_STORAGE_REGISTRY: readonly LocalStorageRegistryEntry[] = [
  {
    key: PREFERENCES_STORAGE_KEY,
    version: PREFERENCES_STORAGE_VERSION,
    owner: "preferences_store",
    legacyKeys: [],
    description: "Client-only preferences (theme, language, layout, health poll)",
  },
  {
    key: ASSETS_STATE_STORAGE_KEY,
    version: ASSETS_STATE_STORAGE_VERSION,
    owner: "assets_provider",
    legacyKeys: [LEGACY_ASSETS_STATE_STORAGE_KEY],
    description: "Persisted assets feature filters and selection state",
  },
] as const;
