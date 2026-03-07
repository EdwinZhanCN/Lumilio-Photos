import { useCallback, useEffect, useMemo } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { useSettingsContext } from "./useSettings";
import {
  type IndexingRepositoryOption,
  useIndexingRepositories,
} from "./useAssetIndexing";

type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

export function getRepositoryDisplayName(
  repository: IndexingRepositoryOption | undefined,
  t: TranslateFn,
): string {
  if (!repository) {
    return t("navbar.repository.all", {
      defaultValue: "All repositories",
    });
  }

  if (repository.isPrimary) {
    return t("navbar.repository.primary", {
      defaultValue: "Primary",
    });
  }

  return repository.name || repository.path;
}

export function useWorkingRepository() {
  const { t } = useI18n();
  const { state, dispatch } = useSettingsContext();
  const repositoriesQuery = useIndexingRepositories();
  const repositories = repositoriesQuery.repositories;
  const workingRepositoryId = state.ui.working_repository_id?.trim() ?? "";

  const selectedRepository = useMemo(
    () =>
      repositories.find((repository) => repository.id === workingRepositoryId),
    [repositories, workingRepositoryId],
  );

  const setWorkingRepositoryId = useCallback(
    (repositoryId?: string | null) => {
      dispatch({
        type: "SET_WORKING_REPOSITORY_ID",
        payload: repositoryId?.trim() || null,
      });
    },
    [dispatch],
  );

  useEffect(() => {
    if (
      !repositoriesQuery.isSuccess ||
      !workingRepositoryId ||
      selectedRepository
    ) {
      return;
    }

    setWorkingRepositoryId(null);
  }, [
    repositoriesQuery.isSuccess,
    selectedRepository,
    setWorkingRepositoryId,
    workingRepositoryId,
  ]);

  const scopeLabel = selectedRepository
    ? getRepositoryDisplayName(selectedRepository, t)
    : workingRepositoryId
      ? repositoriesQuery.isLoading
        ? t("common.loading")
        : t("navbar.repository.unavailable", {
            defaultValue: "Repository options unavailable",
          })
      : t("navbar.repository.all", {
          defaultValue: "All repositories",
        });

  const scopeDescription = selectedRepository?.path
    ? selectedRepository.path
    : workingRepositoryId
      ? t("settings.serverSettings.workingRepositoryUnavailable", {
          defaultValue: "Repository options are temporarily unavailable.",
        })
      : t("settings.serverSettings.workingRepositoryHint", {
          defaultValue:
            "This scope is used by assets, home, map, stats, upload, and ML indexing tools when they support repository filtering.",
        });

  return {
    repositories,
    repositoriesQuery,
    workingRepositoryId,
    scopedRepositoryId: workingRepositoryId || undefined,
    selectedRepository,
    scopeLabel,
    scopeDescription,
    getRepositoryLabel: useCallback(
      (repository: IndexingRepositoryOption) =>
        getRepositoryDisplayName(repository, t),
      [t],
    ),
    setWorkingRepositoryId,
  };
}
