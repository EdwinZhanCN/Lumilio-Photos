import { useCallback, useEffect, useMemo } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { usePreference } from "@/lib/preferences/preferences";
import type { RepositoryOption } from "../../types";
import { getRepositoryDisplayName } from "../../model/repositoryDisplayName";
import { isRepositoryUnavailable } from "../../model/repositoryOptions";
import { useRepositoryOptions } from "../../api/useRepositoryOptions";

export function useWorkingRepository() {
  const { t } = useI18n();
  const [workingRepositoryId, setWorkingRepositoryIdPreference] =
    usePreference("workingRepositoryId");
  const repositoriesQuery = useRepositoryOptions();
  const repositories = repositoriesQuery.repositories;
  const normalizedWorkingRepositoryId = workingRepositoryId?.trim() ?? "";

  const selectedRepository = useMemo(
    () => repositories.find((repository) => repository.id === normalizedWorkingRepositoryId),
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
    if (!repositoriesQuery.isSuccess || repositories.length === 0) {
      return;
    }

    if (selectedRepository) {
      return;
    }

    // Auto-selecting an unreachable repository as the upload target guarantees
    // the next upload is refused, so only fall back to a reachable one. An
    // explicit user choice is left alone even when it goes offline.
    const reachable = repositories.filter((repository) => !isRepositoryUnavailable(repository));
    const fallback = reachable.find((repository) => repository.isPrimary) ?? reachable[0];
    if (fallback && normalizedWorkingRepositoryId !== fallback.id) {
      setWorkingRepositoryId(fallback.id);
    }
  }, [
    repositoriesQuery.isSuccess,
    selectedRepository,
    setWorkingRepositoryId,
    normalizedWorkingRepositoryId,
    repositories,
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
      (repository: RepositoryOption) => getRepositoryDisplayName(repository, t),
      [t],
    ),
    setWorkingRepositoryId,
  };
}
