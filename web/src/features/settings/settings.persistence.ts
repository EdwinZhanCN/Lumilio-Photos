import { getCurrentLanguage, type SupportedLanguage } from "@/lib/i18n.tsx";
import {
  isRecord,
  readVersionedStorageCandidate,
  removeStorageKeys,
  writeVersionedStorageData,
} from "@/lib/settings/storage";
import type { SettingsState } from "./settings.type.ts";
import {
  LEGACY_SETTINGS_STORAGE_KEY,
  SETTINGS_STORAGE_KEY,
  SETTINGS_STORAGE_VERSION,
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

function asBool(value: unknown, fallback: boolean): boolean {
  return typeof value === "boolean" ? value : fallback;
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
  fallback: "compact" | "wide" | "full",
): "compact" | "wide" | "full" {
  return value === "compact" || value === "wide" || value === "full"
    ? value
    : fallback;
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

function defaultedState(base: SettingsState): SettingsState {
  const language = getCurrentLanguage();
  return {
    ...base,
    ui: {
      ...base.ui,
      language,
      region: "other",
      working_repository_id: base.ui.working_repository_id,
      asset_page: {
        layout: base.ui.asset_page?.layout ?? "full",
      },
      upload: {
        max_total_files: base.ui.upload?.max_total_files ?? 100,
        low_power_mode: base.ui.upload?.low_power_mode ?? true,
        chunk_size_mb: base.ui.upload?.chunk_size_mb ?? 24,
        max_concurrent_chunks: base.ui.upload?.max_concurrent_chunks ?? 2,
        use_server_config: base.ui.upload?.use_server_config ?? true,
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
  const upload = isRecord(ui.upload) ? ui.upload : {};
  const server = isRecord(candidate.server) ? candidate.server : {};

  return {
    ui: {
      language: asLanguage(ui.language, defaults.ui.language!),
      region: asRegion(ui.region, defaults.ui.region!),
      working_repository_id: asWorkingRepositoryID(ui.working_repository_id),
      asset_page: {
        layout: asLayout(
          isRecord(ui.asset_page) ? ui.asset_page.layout : undefined,
          defaults.ui.asset_page!.layout,
        ),
      },
      upload: {
        max_total_files: clampInt(
          asNumber(upload.max_total_files),
          1,
          500,
          defaults.ui.upload!.max_total_files,
        ),
        low_power_mode: asBool(
          upload.low_power_mode,
          defaults.ui.upload!.low_power_mode!,
        ),
        chunk_size_mb: clampInt(
          asNumber(upload.chunk_size_mb),
          1,
          128,
          defaults.ui.upload!.chunk_size_mb!,
        ),
        max_concurrent_chunks: clampInt(
          asNumber(upload.max_concurrent_chunks),
          1,
          6,
          defaults.ui.upload!.max_concurrent_chunks!,
        ),
        use_server_config: asBool(
          upload.use_server_config,
          defaults.ui.upload!.use_server_config!,
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
      removeStorageKeys([LEGACY_SETTINGS_STORAGE_KEY]);
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
    removeStorageKeys([LEGACY_SETTINGS_STORAGE_KEY]);
  } catch (error) {
    console.warn("[SettingsProvider] Failed to persist settings", error);
  }
}
