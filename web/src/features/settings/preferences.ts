/**
 * Client-only preferences persisted in localStorage. Applied instantly (no save
 * button). Theme field is owned here but effects live in `@/lib/theme`.
 */
import { useCallback, useEffect, useRef, useState } from "react";
import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { getCurrentLanguage, type SupportedLanguage } from "@/lib/i18n.tsx";
import { DEFAULT_THEME_PREFERENCES, type ThemePreferences } from "@/lib/theme/daisyuiThemes";

export interface AssetPagePreferences {
  layout: "compact" | "full";
  columns: number;
}

export interface Preferences {
  language: SupportedLanguage;
  region: "china" | "other";
  theme: ThemePreferences;
  assetPage: AssetPagePreferences;
  workingRepositoryId?: string;
  browseRepositoryId?: string;
  /** Monitor/health-check polling interval in milliseconds. */
  healthCheckIntervalMs: number;
}

export const DEFAULT_PREFERENCES: Preferences = {
  language: getCurrentLanguage(),
  region: "other",
  theme: DEFAULT_THEME_PREFERENCES,
  assetPage: {
    layout: "full",
    columns: 6,
  },
  workingRepositoryId: undefined,
  browseRepositoryId: undefined,
  healthCheckIntervalMs: 30_000,
};

interface PreferencesState extends Preferences {
  setPreference: <K extends keyof Preferences>(key: K, value: Preferences[K]) => void;
  resetPreferences: () => void;
}

export const usePreferencesStore = create<PreferencesState>()(
  persist(
    (set) => ({
      ...DEFAULT_PREFERENCES,
      setPreference: (key, value) => set({ [key]: value } as Pick<Preferences, typeof key>),
      resetPreferences: () => set({ ...DEFAULT_PREFERENCES }),
    }),
    {
      name: "lumilio.preferences",
      version: 2,
      storage: createJSONStorage(() => localStorage),
      partialize: (state) => ({
        language: state.language,
        region: state.region,
        theme: state.theme,
        assetPage: state.assetPage,
        workingRepositoryId: state.workingRepositoryId,
        browseRepositoryId: state.browseRepositoryId,
        healthCheckIntervalMs: state.healthCheckIntervalMs,
      }),
    },
  ),
);

export function usePreference<K extends keyof Preferences>(
  key: K,
): [Preferences[K], (value: Preferences[K]) => void] {
  const value = usePreferencesStore((s) => s[key]);
  const setPreference = usePreferencesStore((s) => s.setPreference);
  const set = useCallback((next: Preferences[K]) => setPreference(key, next), [key, setPreference]);
  return [value, set];
}

export function useDebouncedPreference<K extends keyof Preferences>(
  key: K,
  delayMs = 300,
): [Preferences[K], (value: Preferences[K]) => void] {
  const [persisted, setPersisted] = usePreference(key);
  const [local, setLocal] = useState<Preferences[K]>(persisted);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    setLocal(persisted);
  }, [persisted]);

  const set = useCallback(
    (next: Preferences[K]) => {
      setLocal(next);
      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(() => setPersisted(next), delayMs);
    },
    [delayMs, setPersisted],
  );

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  return [local, set];
}
