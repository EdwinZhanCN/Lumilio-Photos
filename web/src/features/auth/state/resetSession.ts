import type { QueryClient } from "@tanstack/react-query";
import { usePreferencesStore } from "@/lib/preferences/preferences";
import { invalidateAuthRefresh } from "@/lib/http-commons/client.ts";
import { removeToken } from "@/lib/http-commons/auth.ts";
import {
  ASSETS_STATE_STORAGE_KEY,
  LEGACY_ASSETS_STATE_STORAGE_KEY,
} from "@/lib/settings/registry.ts";
import { removeStorageKeys } from "@/lib/settings/storage.ts";

export interface SessionResetDependencies {
  queryClient: QueryClient;
  resetGlobalState: () => void;
  resetFeatureState: () => void;
}

/** Clear every user-scoped client owner through one ordered session boundary. */
export async function resetSession({
  queryClient,
  resetGlobalState,
  resetFeatureState,
}: SessionResetDependencies): Promise<void> {
  invalidateAuthRefresh();
  removeToken();

  resetFeatureState();
  resetGlobalState();

  usePreferencesStore.setState({
    workingRepositoryId: undefined,
    browseRepositoryId: undefined,
  });
  removeStorageKeys([ASSETS_STATE_STORAGE_KEY, LEGACY_ASSETS_STATE_STORAGE_KEY]);

  await queryClient.cancelQueries();
  queryClient.clear();
}
