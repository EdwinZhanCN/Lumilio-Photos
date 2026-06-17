import { useCallback, useEffect, useMemo } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { usePreference } from "../preferences";
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
  const [workingRepositoryId, setWorkingRepositoryIdPreference] =
    usePreference("workingRepositoryId");
  const repositoriesQuery = useIndexingRepositories();
  const repositories = repositoriesQuery.repositories;
  const normalizedWorkingRepositoryId = workingRepositoryId?.trim() ?? "";

  const selectedRepository = useMemo(
    () =>
      repositories.find(
        (repository) => repository.id === normalizedWorkingRepositoryId,
      ),
    [repositories, normalizedWorkingRepositoryId],
  );

  const setWorkingRepositoryId = useCallback(
    (repositoryId?: string | null) => {
      const normalized = repositoryId?.trim();
      setWorkingRepositoryIdPreference(
        normalized && normalized.length > 0 ? normalized : undefined,
      );
    },
    [setWorkingRepositoryIdPreference],
  );

  useEffect(() => {
    if (
      !repositoriesQuery.isSuccess ||
      !normalizedWorkingRepositoryId ||
      selectedRepository
    ) {
      return;
    }

    setWorkingRepositoryId(null);
  }, [
    repositoriesQuery.isSuccess,
    selectedRepository,
    setWorkingRepositoryId,
    normalizedWorkingRepositoryId,
  ]);

  const scopeLabel = selectedRepository
    ? getRepositoryDisplayName(selectedRepository, t)
    : normalizedWorkingRepositoryId
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
    : normalizedWorkingRepositoryId
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
    workingRepositoryId: normalizedWorkingRepositoryId,
    scopedRepositoryId: normalizedWorkingRepositoryId || undefined,
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
