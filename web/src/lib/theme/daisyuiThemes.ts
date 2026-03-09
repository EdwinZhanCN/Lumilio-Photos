import { parseJSON } from "@/lib/settings/storage";

export type ThemeMode = "light" | "dark";

export const DAISYUI_THEME_ORDER = [
  "light",
  "dark",
  "cupcake",
  "bumblebee",
  "emerald",
  "corporate",
  "synthwave",
  "retro",
  "cyberpunk",
  "valentine",
  "halloween",
  "garden",
  "forest",
  "aqua",
  "lofi",
  "pastel",
  "fantasy",
  "wireframe",
  "black",
  "luxury",
  "dracula",
  "cmyk",
  "autumn",
  "business",
  "acid",
  "lemonade",
  "night",
  "coffee",
  "winter",
  "dim",
  "nord",
  "sunset",
  "caramellatte",
  "abyss",
  "silk",
] as const;

export type DaisyUIThemeName = (typeof DAISYUI_THEME_ORDER)[number];

export const DAISYUI_LIGHT_THEMES = [
  "light",
  "cupcake",
  "bumblebee",
  "emerald",
  "corporate",
  "retro",
  "cyberpunk",
  "valentine",
  "garden",
  "lofi",
  "pastel",
  "fantasy",
  "wireframe",
  "cmyk",
  "autumn",
  "acid",
  "lemonade",
  "winter",
  "nord",
  "caramellatte",
  "silk",
] as const;

export type DaisyUILightThemeName = (typeof DAISYUI_LIGHT_THEMES)[number];

export const DAISYUI_DARK_THEMES = [
  "dark",
  "synthwave",
  "halloween",
  "forest",
  "aqua",
  "black",
  "luxury",
  "dracula",
  "business",
  "night",
  "coffee",
  "dim",
  "sunset",
  "abyss",
] as const;

export type DaisyUIDarkThemeName = (typeof DAISYUI_DARK_THEMES)[number];

export interface ThemePreferences {
  followSystem: boolean;
  mode: ThemeMode;
  themes: {
    light: DaisyUILightThemeName;
    dark: DaisyUIDarkThemeName;
  };
}

export const DEFAULT_THEME_PREFERENCES: ThemePreferences = {
  followSystem: true,
  mode: "light",
  themes: {
    light: "light",
    dark: "night",
  },
};

const lightThemeSet = new Set<string>(DAISYUI_LIGHT_THEMES);
const darkThemeSet = new Set<string>(DAISYUI_DARK_THEMES);

function cloneThemePreferences(
  preferences: ThemePreferences,
): ThemePreferences {
  return {
    followSystem: preferences.followSystem,
    mode: preferences.mode,
    themes: {
      light: preferences.themes.light,
      dark: preferences.themes.dark,
    },
  };
}

export function isThemeMode(value: unknown): value is ThemeMode {
  return value === "light" || value === "dark";
}

export function isLightDaisyUITheme(
  value: unknown,
): value is DaisyUILightThemeName {
  return typeof value === "string" && lightThemeSet.has(value);
}

export function isDarkDaisyUITheme(
  value: unknown,
): value is DaisyUIDarkThemeName {
  return typeof value === "string" && darkThemeSet.has(value);
}

export function normalizeThemePreferences(
  candidate: unknown,
  fallback: ThemePreferences = DEFAULT_THEME_PREFERENCES,
): ThemePreferences {
  const normalizedFallback = cloneThemePreferences(fallback);

  if (typeof candidate !== "object" || candidate === null) {
    return normalizedFallback;
  }

  const maybeTheme = candidate as Partial<ThemePreferences> & {
    followSystem?: unknown;
    themes?: {
      light?: unknown;
      dark?: unknown;
    };
  };
  const hasLegacyManualPreference =
    "mode" in maybeTheme || "themes" in maybeTheme;

  return {
    followSystem:
      typeof maybeTheme.followSystem === "boolean"
        ? maybeTheme.followSystem
        : hasLegacyManualPreference
          ? false
          : normalizedFallback.followSystem,
    mode: isThemeMode(maybeTheme.mode)
      ? maybeTheme.mode
      : normalizedFallback.mode,
    themes: {
      light: isLightDaisyUITheme(maybeTheme.themes?.light)
        ? maybeTheme.themes.light
        : normalizedFallback.themes.light,
      dark: isDarkDaisyUITheme(maybeTheme.themes?.dark)
        ? maybeTheme.themes.dark
        : normalizedFallback.themes.dark,
    },
  };
}

export function resolveThemeNameForMode(
  preferences: ThemePreferences,
  mode: ThemeMode,
): DaisyUIThemeName {
  return mode === "dark" ? preferences.themes.dark : preferences.themes.light;
}

export function getSystemThemeMode(): ThemeMode {
  if (
    typeof window === "undefined" ||
    typeof window.matchMedia !== "function"
  ) {
    return "light";
  }

  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

export function resolveActiveThemeMode(
  preferences: ThemePreferences,
  systemThemeMode: ThemeMode = getSystemThemeMode(),
): ThemeMode {
  return preferences.followSystem ? systemThemeMode : preferences.mode;
}

export function applyThemePreferencesToDocument(
  preferences: ThemePreferences,
  systemThemeMode: ThemeMode = getSystemThemeMode(),
): void {
  if (typeof document === "undefined") {
    return;
  }

  document.documentElement.setAttribute(
    "data-theme",
    resolveThemeNameForMode(
      preferences,
      resolveActiveThemeMode(preferences, systemThemeMode),
    ),
  );
}

export function parseLegacyThemeMode(raw: string | null): ThemeMode | null {
  if (!raw) {
    return null;
  }

  if (isThemeMode(raw)) {
    return raw;
  }

  const parsed = parseJSON(raw);
  if (isThemeMode(parsed)) {
    return parsed;
  }

  if (typeof parsed !== "object" || parsed === null) {
    return null;
  }

  const maybeLegacy = parsed as {
    data?: unknown;
    theme?: unknown;
  };

  if (isThemeMode(maybeLegacy.data)) {
    return maybeLegacy.data;
  }

  if (isThemeMode(maybeLegacy.theme)) {
    return maybeLegacy.theme;
  }

  return null;
}
