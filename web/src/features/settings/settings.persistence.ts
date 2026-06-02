import { getCurrentLanguage, type SupportedLanguage } from "@/lib/i18n.tsx";
import {
  isRecord,
  readVersionedStorageCandidate,
  removeStorageKeys,
  writeVersionedStorageData,
} from "@/lib/settings/storage";
import {
  DEFAULT_THEME_PREFERENCES,
  normalizeThemePreferences,
  parseLegacyThemeMode,
  type ThemeMode,
} from "@/lib/theme/daisyuiThemes";
import type { SettingsState } from "./settings.type.ts";
import {
  LEGACY_THEME_STORAGE_KEY,
  LEGACY_SETTINGS_STORAGE_KEY,
  SETTINGS_STORAGE_KEY,
  SETTINGS_STORAGE_VERSION,
  THEME_STORAGE_KEY,
} from "./settings.registry";

function asNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value)
    ? value
    : undefined;
}

function clampNumber(
  value: number | undefined,
  min: number,
  max: number,
  fallback: number,
): number {
  if (value === undefined) return fallback;
  return Math.min(max, Math.max(min, value));
}

function clampInt(
  value: number | undefined,
  min: number,
  max: number,
  fallback: number,
): number {
  const n = value === undefined ? undefined : Math.floor(value);
  return Math.floor(clampNumber(n, min, max, fallback));
}

function asLanguage(
  value: unknown,
  fallback: SupportedLanguage,
): SupportedLanguage {
  return value === "en" || value === "zh" ? value : fallback;
}

function asRegion(
  value: unknown,
  fallback: "china" | "other",
): "china" | "other" {
  return value === "china" || value === "other" ? value : fallback;
}

function asLayout(
  value: unknown,
  fallback: "compact" | "full",
): "compact" | "full" {
  if (value === "compact" || value === "full") {
    return value;
  }

  if (value === "wide") {
    return "full";
  }

  return fallback;
}

function asWorkingRepositoryID(value: unknown): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const normalized = value.trim();
  if (!normalized) {
    return undefined;
  }

  return /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
    normalized,
  )
    ? normalized
    : undefined;
}

function resolveLegacyThemeMode(): ThemeMode | null {
  if (typeof localStorage === "undefined") {
    return null;
  }

  return (
    parseLegacyThemeMode(localStorage.getItem(THEME_STORAGE_KEY)) ??
    parseLegacyThemeMode(localStorage.getItem(LEGACY_THEME_STORAGE_KEY))
  );
}

function defaultedState(base: SettingsState): SettingsState {
  const language = getCurrentLanguage();
  const baseTheme = normalizeThemePreferences(
    base.ui.theme,
    DEFAULT_THEME_PREFERENCES,
  );
  const legacyThemeMode = resolveLegacyThemeMode();
  const theme = normalizeThemePreferences(
    legacyThemeMode
      ? {
          followSystem: false,
          mode: legacyThemeMode,
          themes: baseTheme.themes,
        }
      : null,
    baseTheme,
  );

  return {
    ...base,
    ui: {
      ...base.ui,
      language,
      region: "other",
      working_repository_id: base.ui.working_repository_id,
      theme,
      asset_page: {
        layout: base.ui.asset_page?.layout ?? "full",
        columns: base.ui.asset_page?.columns ?? 6,
      },
    },
    server: {
      update_timespan: base.server.update_timespan,
    },
  };
}

function sanitizeSettings(
  candidate: unknown,
  base: SettingsState,
): SettingsState {
  const defaults = defaultedState(base);
  if (!isRecord(candidate)) {
    return defaults;
  }

  const ui = isRecord(candidate.ui) ? candidate.ui : {};
  const server = isRecord(candidate.server) ? candidate.server : {};

  return {
    ui: {
      language: asLanguage(ui.language, defaults.ui.language!),
      region: asRegion(ui.region, defaults.ui.region!),
      working_repository_id: asWorkingRepositoryID(ui.working_repository_id),
      theme: normalizeThemePreferences(ui.theme, defaults.ui.theme),
      asset_page: {
        layout: asLayout(
          isRecord(ui.asset_page) ? ui.asset_page.layout : undefined,
          defaults.ui.asset_page!.layout,
        ),
        columns: clampInt(
          asNumber(isRecord(ui.asset_page) ? ui.asset_page.columns : undefined),
          4,
          10,
          defaults.ui.asset_page!.columns,
        ),
      },
    },
    server: {
      update_timespan: clampNumber(
        asNumber(server.update_timespan),
        1,
        50,
        defaults.server.update_timespan,
      ),
    },
  };
}

function writeSettingsEnvelope(state: SettingsState): void {
  writeVersionedStorageData<SettingsState>(
    SETTINGS_STORAGE_KEY,
    SETTINGS_STORAGE_VERSION,
    state,
  );
}

export function resolveInitialSettingsState(
  base: SettingsState,
): SettingsState {
  const defaults = defaultedState(base);
  const readResult = readVersionedStorageCandidate({
    key: SETTINGS_STORAGE_KEY,
    version: SETTINGS_STORAGE_VERSION,
    legacyKeys: [LEGACY_SETTINGS_STORAGE_KEY],
  });

  if (readResult.candidate !== null) {
    const normalized = sanitizeSettings(readResult.candidate, defaults);
    if (readResult.needsRewrite) {
      writeSettingsEnvelope(normalized);
      removeStorageKeys([
        LEGACY_SETTINGS_STORAGE_KEY,
        THEME_STORAGE_KEY,
        LEGACY_THEME_STORAGE_KEY,
      ]);
    }
    return normalized;
  }

  return defaults;
}

export function persistSettingsState(state: SettingsState): void {
  if (typeof localStorage === "undefined") {
    return;
  }

  try {
    writeSettingsEnvelope(state);
    removeStorageKeys([
      LEGACY_SETTINGS_STORAGE_KEY,
      THEME_STORAGE_KEY,
      LEGACY_THEME_STORAGE_KEY,
    ]);
  } catch (error) {
    console.warn("[SettingsProvider] Failed to persist settings", error);
  }
}
