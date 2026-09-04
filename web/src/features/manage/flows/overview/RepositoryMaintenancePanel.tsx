import { useCallback, useMemo } from "react";
import { useStartRepositoryCloudImport } from "@/features/cloud";
import { useDetectDuplicates } from "@/features/collections";
import { useEventRebuild, useEventRebuildStatus } from "@/features/events";
import { useRebuildPeopleClusters } from "@/features/people";
import {
  getStorageEntityDisplayName,
  RepositoryGrid,
  type RepositoryOption,
  useRepositoryOptions,
  useRepositoryScan,
} from "@/features/repositories";
import { useMessage } from "@/features/notifications";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import { localizeAPIProblem } from "@/lib/http-commons/problem";

export default function RepositoryMaintenancePanel() {
  const { t } = useI18n();
  const showMessage = useMessage();
  const repositoriesQuery = useRepositoryOptions();
  const repositories = repositoriesQuery.repositories;
  const { scanRepository, scanRepositories, scanningIds, detectStacks, detectingIds, isScanning } =
    useRepositoryScan();
  const detectDuplicatesMutation = useDetectDuplicates();
  const duplicateScanningRepositoryId =
    detectDuplicatesMutation.isPending && detectDuplicatesMutation.variables
      ? detectDuplicatesMutation.variables.repositoryId
      : undefined;
  const locationRebuildMutation = $api.useMutation("post", "/api/v1/locations/rebuild");
  const cloudImportMutation = useStartRepositoryCloudImport();
  // People span repositories, so the rebuild is a library-wide job: no
  // per-repository target here on purpose.
  const { rebuildPeople, isRebuilding: isRebuildingPeople } = useRebuildPeopleClusters();
  // Events share the same user-owned topology as people, so this maintenance
  // action also intentionally rebuilds across every repository.
  const { rebuild: rebuildEvents, isRebuilding: isRebuildingEvents } = useEventRebuild();
  // Keep the maintenance surface truthful while an asynchronous owner-wide
  // rebuild is queued or running. The status hook stops polling when settled.
  useEventRebuildStatus();

  const repositoryIds = useMemo(
    () => repositories.map((repository) => repository.id).filter(Boolean),
    [repositories],
  );

  const handleScanRepository = useCallback(
    async (repository: RepositoryOption) => {
      try {
        const result = await scanRepository(repository.id);
        showMessage(
          result.coalesced ? "info" : "success",
          result.coalesced
            ? t(
                "manage.repositories.scanCoalesced",
                "A scan is already active for {{name}}; this request joined the existing operation.",
                { name: getStorageEntityDisplayName(repository, t) },
              )
            : t(
                "manage.repositories.scanQueued",
                "Scan queued for {{name}}. You can keep using Lumilio while it runs.",
                { name: getStorageEntityDisplayName(repository, t) },
              ),
        );
      } catch (error) {
        showMessage("error", localizeAPIProblem(error, t, t("manage.repositories.scanFailed")));
      }
    },
    [scanRepository, showMessage, t],
  );

  const handleDetectStacks = useCallback(
    async (repository: RepositoryOption) => {
      try {
        const created = await detectStacks(repository.id);
        showMessage(
          "success",
          t("manage.repositories.detectStacksCompleted", {
            name: getStorageEntityDisplayName(repository, t),
            count: created,
          }),
        );
      } catch (error) {
        showMessage(
          "error",
          localizeAPIProblem(error, t, t("manage.repositories.detectStacksFailed")),
        );
      }
    },
    [detectStacks, showMessage, t],
  );

  const handleDuplicateScan = useCallback(
    async (repository: RepositoryOption) => {
      try {
        const result = await detectDuplicatesMutation.mutateAsync({
          repositoryId: repository.id,
        });
        showMessage(
          "success",
          t("duplicates.scanSuccess", {
            groups: result.groups ?? 0,
            exact: result.exact_groups ?? 0,
            phash: result.phash_groups ?? 0,
            mixed: result.mixed_groups ?? 0,
          }),
        );
      } catch (error) {
        showMessage(
          "error",
          t("duplicates.scanError", {
            message: localizeAPIProblem(error, t, t("manage.repositories.duplicateScanFailed")),
          }),
        );
      }
    },
    [detectDuplicatesMutation, showMessage, t],
  );

  const rebuildingLocationId =
    locationRebuildMutation.isPending && locationRebuildMutation.variables
      ? ((
          locationRebuildMutation.variables as {
            body?: { repository_id?: string };
          }
        )?.body?.repository_id ?? null)
      : null;

  const handleLocationRebuild = useCallback(
    async (repository: RepositoryOption) => {
      try {
        await locationRebuildMutation.mutateAsync({
          body: { repository_id: repository.id },
        });
        showMessage("success", t("manage.repositories.rebuildLocationQueued"));
      } catch (error) {
        showMessage("error", localizeAPIProblem(error, t, t("home.errors.unknown")));
      }
    },
    [locationRebuildMutation, showMessage, t],
  );

  const cloudImportingRepositoryId =
    cloudImportMutation.isPending && cloudImportMutation.variables
      ? (cloudImportMutation.variables as { params?: { path?: { id?: string } } }).params?.path?.id
      : undefined;

  const handleCloudImport = useCallback(
    async (repository: RepositoryOption) => {
      try {
        await cloudImportMutation.mutateAsync({
          params: {
            path: {
              id: repository.id,
            },
          },
        });
        showMessage(
          "success",
          t("manage.repositories.cloudImportStarted", {
            name: getStorageEntityDisplayName(repository, t),
          }),
        );
      } catch (error) {
        showMessage(
          "error",
          localizeAPIProblem(error, t, t("manage.repositories.cloudImportFailed")),
        );
      }
    },
    [cloudImportMutation, showMessage, t],
  );

  const handleRebuildPeople = useCallback(async () => {
    try {
      const result = await rebuildPeople();
      showMessage(
        "success",
        t("people.rebuild.success", {
          clusters: result?.clusters_total ?? 0,
          faces: result?.clustered_faces ?? 0,
          noise: result?.noise_faces ?? 0,
        }),
      );
    } catch (error) {
      showMessage(
        "error",
        t("people.rebuild.error", {
          message: localizeAPIProblem(error, t, t("home.errors.unknown")),
        }),
      );
    }
  }, [rebuildPeople, showMessage, t]);

  const handleRebuildEvents = useCallback(async () => {
    try {
      const result = await rebuildEvents();
      showMessage(
        "success",
        t("events.rebuild.queued", "Event rebuild queued ({{run}}).", {
          run: result?.run_id?.slice(0, 8) ?? "pending",
        }),
      );
    } catch (error) {
      showMessage(
        "error",
        t("events.rebuild.error", "Failed to rebuild Events: {{message}}", {
          message: localizeAPIProblem(error, t, t("home.errors.unknown")),
        }),
      );
    }
  }, [rebuildEvents, showMessage, t]);

  const handleScanAll = useCallback(async () => {
    try {
      await scanRepositories(repositoryIds);
      showMessage(
        "success",
        t("manage.repositories.scanAllQueued", {
          count: repositoryIds.length,
        }),
      );
    } catch (error) {
      showMessage("error", localizeAPIProblem(error, t, t("manage.repositories.scanFailed")));
    }
  }, [repositoryIds, scanRepositories, showMessage, t]);

  return (
    <RepositoryGrid
      repositories={repositories}
      repositoryIds={repositoryIds}
      isLoading={repositoriesQuery.isLoading}
      isError={repositoriesQuery.isError}
      isScanning={isScanning}
      isRebuildingEvents={isRebuildingEvents}
      isRebuildingPeople={isRebuildingPeople}
      scanningIds={scanningIds}
      detectingIds={detectingIds}
      duplicateScanningRepositoryId={duplicateScanningRepositoryId}
      rebuildingLocationId={rebuildingLocationId}
      cloudImportingRepositoryId={cloudImportingRepositoryId}
      onScanRepository={handleScanRepository}
      onDetectStacks={handleDetectStacks}
      onDuplicateScan={handleDuplicateScan}
      onLocationRebuild={handleLocationRebuild}
      onCloudImport={handleCloudImport}
      onScanAll={handleScanAll}
      onRebuildEvents={handleRebuildEvents}
      onRebuildPeople={handleRebuildPeople}
    />
  );
}
