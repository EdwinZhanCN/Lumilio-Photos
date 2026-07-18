import { useCallback, useMemo } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { usePreference } from "@/lib/preferences/preferences";
import type { RepositoryOption } from "../../types";
import { getRepositoryDisplayName } from "../../model/repositoryDisplayName";
import { useRepositoryOptions } from "../../api/useRepositoryOptions";

type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

export function useBrowseScope() {
  const { t } = useI18n();
  const [browseRepositoryId, setBrowseRepositoryIdPreference] = usePreference("browseRepositoryId");
  const repositoriesQuery = useRepositoryOptions();
  const repositories = repositoriesQuery.repositories;
  const normalizedBrowseId = browseRepositoryId?.trim() ?? "";

  const selectedRepository = useMemo(
    () => repositories.find((repository) => repository.id === normalizedBrowseId),
    [repositories, normalizedBrowseId],
  );

  const setBrowseRepositoryId = useCallback(
    (repositoryId?: string | null) => {
      const normalized = repositoryId?.trim();
      setBrowseRepositoryIdPreference(normalized && normalized.length > 0 ? normalized : undefined);
    },
    [setBrowseRepositoryIdPreference],
  );

  const scopeLabel = selectedRepository
    ? getRepositoryDisplayName(selectedRepository, t as TranslateFn)
    : normalizedBrowseId
      ? repositoriesQuery.isLoading
        ? t("common.loading")
        : t("navbar.repository.unavailable", { defaultValue: "Repository options unavailable" })
      : t("navbar.repository.all", { defaultValue: "All repositories" });

  return {
    repositories,
    repositoriesQuery,
    browseRepositoryId: normalizedBrowseId,
    scopedRepositoryId: normalizedBrowseId || undefined,
    selectedRepository,
    scopeLabel,
    getRepositoryLabel: useCallback(
      (repository: RepositoryOption) => getRepositoryDisplayName(repository, t as TranslateFn),
      [t],
    ),
    setBrowseRepositoryId,
  };
}
