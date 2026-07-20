import { beforeEach, describe, expect, it } from "vitest";
import { PREFERENCES_STORAGE_KEY, PREFERENCES_STORAGE_VERSION } from "@/lib/settings/registry";
import {
  DEFAULT_PREFERENCES,
  usePreferencesStore as useSharedPreferencesStore,
} from "@/lib/preferences/preferences";
import { usePreferencesStore } from "./preferences";

describe("persisted preferences", () => {
  beforeEach(() => {
    localStorage.clear();
    usePreferencesStore.setState({ ...DEFAULT_PREFERENCES });
  });

  it("preserves the Settings export and localStorage envelope", () => {
    expect(usePreferencesStore).toBe(useSharedPreferencesStore);

    usePreferencesStore.getState().setPreference("region", "china");
    usePreferencesStore.getState().setPreference("workingRepositoryId", "repository-a");
    usePreferencesStore.getState().setPreference("browseRepositoryId", "repository-b");

    const persisted = JSON.parse(localStorage.getItem(PREFERENCES_STORAGE_KEY) ?? "null") as {
      state: Record<string, unknown>;
      version: number;
    } | null;

    expect(persisted?.version).toBe(PREFERENCES_STORAGE_VERSION);
    expect(persisted?.state).toMatchObject({
      region: "china",
      workingRepositoryId: "repository-a",
      browseRepositoryId: "repository-b",
    });
    expect(Object.keys(persisted?.state ?? {}).sort()).toEqual(
      [
        "assetPage",
        "browseRepositoryId",
        "healthCheckIntervalMs",
        "language",
        "region",
        "theme",
        "workingRepositoryId",
      ].sort(),
    );
  });
});
