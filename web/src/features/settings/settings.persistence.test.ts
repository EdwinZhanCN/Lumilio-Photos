import { beforeEach, describe, expect, it, vi } from "vite-plus/test";
import type { SettingsState } from "./settings.type.ts";
import {
  persistSettingsState,
  resolveInitialSettingsState,
} from "./settings.persistence";
import {
  LEGACY_THEME_STORAGE_KEY,
  LEGACY_SETTINGS_STORAGE_KEY,
  SETTINGS_STORAGE_KEY,
  SETTINGS_STORAGE_VERSION,
  THEME_STORAGE_KEY,
} from "./settings.registry";

vi.mock("@/lib/i18n.tsx", () => ({
  getCurrentLanguage: () => "en",
}));

const baseState: SettingsState = {
  ui: {
    language: "en",
    region: "other",
    working_repository_id: undefined,
    theme: {
      followSystem: true,
      mode: "light",
      themes: {
        light: "light",
        dark: "night",
      },
    },
    asset_page: { layout: "full", columns: 6 },
  },
  server: {
    update_timespan: 5,
  },
};

function createLocalStorageMock() {
  const store = new Map<string, string>();
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => {
      store.set(key, String(value));
    },
    removeItem: (key: string) => {
      store.delete(key);
    },
    clear: () => {
      store.clear();
    },
  };
}

describe("settings.persistence", () => {
  beforeEach(() => {
    const storage = createLocalStorageMock();
    Object.defineProperty(window, "localStorage", {
      value: storage,
      configurable: true,
    });
    Object.defineProperty(globalThis, "localStorage", {
      value: storage,
      configurable: true,
    });
  });

  it("returns sane defaults when storage is empty", () => {
    const state = resolveInitialSettingsState(baseState);

    expect(state.ui.region).toBe("other");
    expect(state.ui.working_repository_id).toBeUndefined();
    expect(state.ui.theme.followSystem).toBe(true);
    expect(state.ui.theme.mode).toBe("light");
    expect(state.ui.theme.themes.light).toBe("light");
    expect(state.ui.theme.themes.dark).toBe("night");
    expect(state.ui.asset_page?.columns).toBe(6);
    expect(state.server.update_timespan).toBe(5);
  });

  it("migrates legacy v1 settings key to versioned envelope", () => {
    localStorage.setItem(
      THEME_STORAGE_KEY,
      JSON.stringify({
        version: 1,
        data: "dark",
      }),
    );
    localStorage.setItem(
      LEGACY_SETTINGS_STORAGE_KEY,
      JSON.stringify({
        ui: {
          language: "zh",
          region: "china",
          working_repository_id: "550e8400-e29b-41d4-a716-446655440000",
          asset_page: {
            layout: "wide",
            columns: 99,
          },
        },
        server: { update_timespan: 100 },
      }),
    );

    const state = resolveInitialSettingsState(baseState);
    expect(state.ui.language).toBe("zh");
    expect(state.ui.region).toBe("china");
    expect(state.ui.working_repository_id).toBe(
      "550e8400-e29b-41d4-a716-446655440000",
    );
    expect(state.ui.theme.mode).toBe("dark");
    expect(state.ui.theme.followSystem).toBe(false);
    expect(state.ui.theme.themes.light).toBe("light");
    expect(state.ui.theme.themes.dark).toBe("night");
    expect(state.ui.asset_page?.layout).toBe("full");
    expect(state.ui.asset_page?.columns).toBe(10);
    expect(state.server.update_timespan).toBe(50);
    expect(localStorage.getItem(LEGACY_SETTINGS_STORAGE_KEY)).toBeNull();
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBeNull();
    expect(localStorage.getItem(LEGACY_THEME_STORAGE_KEY)).toBeNull();

    const raw = localStorage.getItem(SETTINGS_STORAGE_KEY);
    expect(raw).not.toBeNull();

    const parsed = JSON.parse(raw!);
    expect(parsed.version).toBe(SETTINGS_STORAGE_VERSION);
    expect(parsed.data.ui.region).toBe("china");
  });

  it("self-heals malformed payload to defaults", () => {
    localStorage.setItem(SETTINGS_STORAGE_KEY, "{bad json");

    const state = resolveInitialSettingsState(baseState);
    expect(state.ui.region).toBe("other");
    expect(state.ui.working_repository_id).toBeUndefined();
    expect(state.server.update_timespan).toBe(5);
  });

  it("adopts standalone legacy theme mode and rewrites into settings storage", () => {
    localStorage.setItem(THEME_STORAGE_KEY, JSON.stringify("dark"));

    const state = resolveInitialSettingsState(baseState);
    expect(state.ui.theme.followSystem).toBe(false);
    expect(state.ui.theme.mode).toBe("dark");
    expect(state.ui.theme.themes.light).toBe("light");
    expect(state.ui.theme.themes.dark).toBe("night");

    persistSettingsState(state);

    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBeNull();
    expect(localStorage.getItem(LEGACY_THEME_STORAGE_KEY)).toBeNull();

    const raw = localStorage.getItem(SETTINGS_STORAGE_KEY);
    expect(raw).not.toBeNull();

    const parsed = JSON.parse(raw!);
    expect(parsed.data.ui.theme.followSystem).toBe(false);
    expect(parsed.data.ui.theme.mode).toBe("dark");
  });

  it("persists settings in versioned envelope format", () => {
    const state = resolveInitialSettingsState(baseState);
    const next = {
      ...state,
      server: { update_timespan: 7.5 },
      ui: {
        ...state.ui,
        region: "china" as const,
        working_repository_id: "550e8400-e29b-41d4-a716-446655440000",
        theme: {
          ...state.ui.theme,
          followSystem: false,
          mode: "dark" as const,
          themes: {
            ...state.ui.theme.themes,
            dark: "dracula" as const,
          },
        },
      },
    };

    persistSettingsState(next);

    const raw = localStorage.getItem(SETTINGS_STORAGE_KEY);
    expect(raw).not.toBeNull();

    const parsed = JSON.parse(raw!);
    expect(parsed.version).toBe(SETTINGS_STORAGE_VERSION);
    expect(parsed.data.server.update_timespan).toBe(7.5);
    expect(parsed.data.ui.region).toBe("china");
    expect(parsed.data.ui.working_repository_id).toBe(
      "550e8400-e29b-41d4-a716-446655440000",
    );
    expect(parsed.data.ui.theme.followSystem).toBe(false);
    expect(parsed.data.ui.theme.mode).toBe("dark");
    expect(parsed.data.ui.theme.themes.dark).toBe("dracula");
  });
});
