import { useCallback } from "react";
import { useMessage } from "@/features/notifications";
import { useBrowseScope, useRepositoryScan } from "@/features/repositories";
import { useI18n } from "@/lib/i18n";

export function useAssetsPageHeaderScan() {
  const { t } = useI18n();
  const showMessage = useMessage();
  const { repositories, selectedRepository, scopeLabel } = useBrowseScope();
  const { scanRepositories, isScanning } = useRepositoryScan();

  const handleScanCurrentLibrary = useCallback(async () => {
    const targetRepositoryIds = selectedRepository
      ? [selectedRepository.id]
      : repositories.map((repository) => repository.id).filter(Boolean);

    if (targetRepositoryIds.length === 0) {
      showMessage("info", t("assets.assetsPageHeader.scan.noRepositories"));
      return;
    }

    try {
      await scanRepositories(targetRepositoryIds);
      showMessage(
        "success",
        selectedRepository
          ? t("assets.assetsPageHeader.scan.currentQueued", {
              name: scopeLabel,
            })
          : t("assets.assetsPageHeader.scan.allQueued", {
              count: targetRepositoryIds.length,
            }),
      );
    } catch (error) {
      showMessage(
        "error",
        error instanceof Error ? error.message : t("assets.assetsPageHeader.scan.failed"),
      );
    }
  }, [repositories, scanRepositories, scopeLabel, selectedRepository, showMessage, t]);

  return {
    handleScanCurrentLibrary,
    isScanning,
    repositoriesLength: repositories.length,
    scopeLabel,
  };
}
