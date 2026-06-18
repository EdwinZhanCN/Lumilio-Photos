import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { Link } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  Cloud,
  CloudDownload,
  Copy,
  Ellipsis,
  Folder,
  FolderPlus,
  Layers,
  MapPin,
  Plus,
  RefreshCcw,
  RefreshCcwDot,
  X,
} from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { $api } from "@/lib/http-commons/queryClient";
import {
  type IndexingRepositoryOption,
  useIndexingRepositories,
} from "@/features/settings/hooks/useAssetIndexing";
import { getRepositoryDisplayName } from "@/features/settings/hooks/useWorkingRepository";
import { useRepositoryScan } from "@/features/manage/hooks/useRepositoryScan";
import { useDetectDuplicates } from "@/features/collections/hooks/useDuplicates";
import {
  useCloudCredentials,
  useRepositoryCloudStatus,
  useStartRepositoryCloudImport,
} from "@/features/settings/hooks/useCloudSync";

const getViewerTimeZone = () =>
  typeof Intl !== "undefined"
    ? Intl.DateTimeFormat().resolvedOptions().timeZone
    : "UTC";

function useRepositoryAssetCount(repositoryId: string) {
  const request = useMemo(
    () => ({
      filter: {
        repository_id: repositoryId,
      },
      pagination: {
        limit: 1,
        offset: 0,
      },
      sort_by: "recently_added" as const,
      viewer_timezone: getViewerTimeZone(),
    }),
    [repositoryId],
  );

  const query = $api.useQuery(
    "post",
    "/api/v1/assets/list",
    {
      body: request,
    },
    {
      enabled: Boolean(repositoryId),
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  );

  return {
    ...query,
    assetCount: (query.data?.total_assets ?? 0) as number,
  };
}

function RepositoryCard({
  repository,
  isScanning,
  isDetecting,
  isDuplicateScanning,
  isRebuildingLocation,
  isCloudImporting,
  onScan,
  onDetectStacks,
  onDuplicateScan,
  onLocationRebuild,
  onCloudImport,
}: {
  repository: IndexingRepositoryOption;
  isScanning: boolean;
  isDetecting: boolean;
  isDuplicateScanning: boolean;
  isRebuildingLocation: boolean;
  isCloudImporting: boolean;
  onScan: (repository: IndexingRepositoryOption) => void;
  onDetectStacks: (repository: IndexingRepositoryOption) => void;
  onDuplicateScan: (repository: IndexingRepositoryOption) => void;
  onLocationRebuild: (repository: IndexingRepositoryOption) => void;
  onCloudImport: (repository: IndexingRepositoryOption) => void;
}) {
  const { t } = useI18n();
  const countQuery = useRepositoryAssetCount(repository.id);
  const cloudStatusQuery = useRepositoryCloudStatus(repository.id);
  const cloudStatus = cloudStatusQuery.data;
  const latestRun = cloudStatus?.latest_run;
  const name = getRepositoryDisplayName(repository, t);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const isBusy =
    isScanning || isDetecting || isDuplicateScanning || isRebuildingLocation || isCloudImporting;
  const hasCloudBinding = Boolean(cloudStatus?.credential);
  const latestRunStatus = latestRun?.status;

  useEffect(() => {
    if (!menuOpen) return;

    const handlePointerDown = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };

    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [menuOpen]);

  return (
    <article className="rounded-lg border border-base-300 bg-base-100 p-4 shadow-sm transition hover:border-primary/30">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex size-12 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Folder size={24} strokeWidth={1.8} />
          </div>
          <div className="min-w-0">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <h3 className="truncate text-sm font-semibold text-base-content">
                {name}
              </h3>
              {repository.isPrimary && (
                <span className="badge badge-primary badge-sm">
                  {t("manage.repositories.primaryBadge")}
                </span>
              )}
	      {hasCloudBinding && (
	        <span className="badge badge-info badge-sm gap-1">
	          <Cloud size={12} />
	          {t("manage.repositories.sourceCloud")}
	        </span>
	      )}
            </div>
            <p
              className="mt-1 truncate text-xs text-base-content/55"
              title={repository.path}
            >
              {repository.path}
            </p>
          </div>
        </div>

        <div className="relative shrink-0" ref={menuRef}>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle"
            onClick={() => setMenuOpen((current) => !current)}
            title={t("manage.repositories.actionsMenu", {
              name,
            })}
            aria-label={t("manage.repositories.actionsMenu", {
              name,
            })}
            aria-expanded={menuOpen}
          >
            <Ellipsis size={16} />
          </button>

          {menuOpen && (
            <div className="absolute right-0 top-full z-20 mt-2 w-52 overflow-hidden rounded-2xl border border-base-300 bg-base-100 p-2 shadow-xl">
              <button
                type="button"
                className="flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition-colors hover:bg-base-200 disabled:cursor-not-allowed disabled:opacity-60"
                onClick={() => {
                  setMenuOpen(false);
                  onScan(repository);
                }}
                disabled={isBusy}
              >
                {isScanning ? (
                  <span className="loading loading-spinner loading-xs" />
                ) : (
                  <RefreshCcw size={16} className="text-base-content/70" />
                )}
                <span>
                  {t("manage.repositories.rescanRepository", {
                    name,
                  })}
                </span>
              </button>

              <button
                type="button"
                className="mt-1 flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition-colors hover:bg-base-200 disabled:cursor-not-allowed disabled:opacity-60"
                onClick={() => {
                  setMenuOpen(false);
                  onDetectStacks(repository);
                }}
                disabled={isBusy}
              >
                {isDetecting ? (
                  <span className="loading loading-spinner loading-xs" />
                ) : (
                  <Layers size={16} className="text-base-content/70" />
                )}
                <span>
                  {t("manage.repositories.detectStacks", {
                    name,
                  })}
                </span>
              </button>

              <button
                type="button"
                className="mt-1 flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition-colors hover:bg-base-200 disabled:cursor-not-allowed disabled:opacity-60"
                onClick={() => {
                  setMenuOpen(false);
                  onDuplicateScan(repository);
                }}
                disabled={isBusy}
              >
                {isDuplicateScanning ? (
                  <span className="loading loading-spinner loading-xs" />
                ) : (
                  <Copy size={16} className="text-base-content/70" />
                )}
                <span>{t("manage.repositories.duplicateScan")}</span>
              </button>
              <button
                type="button"
                className="mt-1 flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition-colors hover:bg-base-200 disabled:cursor-not-allowed disabled:opacity-60"
                onClick={() => {
                  setMenuOpen(false);
                  onLocationRebuild(repository);
                }}
                disabled={isBusy}
              >
                {isRebuildingLocation ? (
                  <span className="loading loading-spinner loading-xs" />
                ) : (
                  <MapPin size={16} className="text-base-content/70" />
                )}
                <span>{t("manage.repositories.rebuildLocation")}</span>
              </button>
              {hasCloudBinding && (
                <button
                  type="button"
                  className="mt-1 flex w-full items-center gap-3 rounded-xl px-3 py-2 text-left text-sm transition-colors hover:bg-base-200 disabled:cursor-not-allowed disabled:opacity-60"
                  onClick={() => {
                    setMenuOpen(false);
                    onCloudImport(repository);
                  }}
                  disabled={isBusy || latestRunStatus === "running" || latestRunStatus === "queued"}
                >
                  {isCloudImporting || latestRunStatus === "running" || latestRunStatus === "queued" ? (
                    <span className="loading loading-spinner loading-xs" />
                  ) : (
                    <CloudDownload size={16} className="text-base-content/70" />
                  )}
	                  <span>{t("manage.repositories.importFromCloud")}</span>
	                </button>
              )}
            </div>
          )}
        </div>
      </div>

      <div className="mt-4 flex items-end justify-between gap-3 border-t border-base-200 pt-3">
        <div>
          <div className="text-2xl font-semibold tabular-nums">
            {countQuery.isLoading ? (
              <span className="loading loading-dots loading-sm" />
            ) : (
              countQuery.assetCount.toLocaleString()
            )}
          </div>
          <div className="text-xs text-base-content/55">
            {t("manage.repositories.assetCount")}
          </div>
        </div>
        {hasCloudBinding && latestRun && (
          <div className="text-right text-xs text-base-content/60">
            <div className="font-medium capitalize text-base-content/80">{latestRun.status}</div>
            <div>
              {(latestRun.imported_count ?? 0).toLocaleString()} imported ·{" "}
              {(latestRun.failed_count ?? 0).toLocaleString()} failed
            </div>
          </div>
        )}
      </div>
    </article>
  );
}

function AddRepositoryModal({
  isOpen,
  onClose,
}: {
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const queryClient = useQueryClient();
  const createMutation = $api.useMutation("post", "/api/v1/repositories");
  const credentialsQuery = useCloudCredentials();
  const [name, setName] = useState("");
  const [source, setSource] = useState<"local" | "cloud">("local");
  const [credentialId, setCredentialId] = useState("");

  const credentials = useMemo(
    () => (credentialsQuery.data?.credentials ?? []).filter((item) => item.status === "connected"),
    [credentialsQuery.data],
  );

  const handleClose = useCallback(() => {
    if (createMutation.isPending) return;
    setName("");
    setSource("local");
    setCredentialId("");
    onClose();
  }, [createMutation.isPending, onClose]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const trimmedName = name.trim();
      if (!trimmedName || createMutation.isPending) return;
      if (source === "cloud" && !credentialId) return;

      try {
        const response = await createMutation.mutateAsync({
          body: {
            name: trimmedName,
            cloud_credential_id: source === "cloud" ? credentialId : undefined,
          },
        });
        await Promise.all([
          queryClient.invalidateQueries({
            queryKey: ["get", "/api/v1/assets/indexing/repositories"],
          }),
          queryClient.invalidateQueries({
            queryKey: ["post", "/api/v1/assets/list"],
          }),
          queryClient.invalidateQueries({
            queryKey: ["post", "/api/v1/assets/search"],
          }),
        ]);
        showMessage(
          response.cloud_import_error ? "info" : "success",
          response.cloud_import_error
            ? t("manage.repositories.cloudImportCreatePartial", {
                error: response.cloud_import_error,
              })
            : source === "cloud"
              ? t("manage.repositories.cloudImportCreateSuccess", {
                  name: trimmedName,
                })
              : t("manage.repositories.createSuccess", { name: trimmedName }),
        );
        setName("");
        setSource("local");
        setCredentialId("");
        onClose();
      } catch (error) {
        showMessage(
          "error",
          error instanceof Error
            ? error.message
            : t("manage.repositories.createFailed"),
        );
      }
    },
    [createMutation, credentialId, name, onClose, queryClient, showMessage, source, t],
  );

  if (!isOpen) return null;

  return (
    <div className="modal modal-open z-50">
      <div className="modal-box max-w-md">
        <div className="mb-5 flex items-start justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <FolderPlus size={20} />
            </div>
            <div>
              <h3 className="text-base font-semibold">
                {t("manage.repositories.createTitle")}
              </h3>
            </div>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle"
            onClick={handleClose}
            disabled={createMutation.isPending}
            aria-label={t("common.close", { defaultValue: "Close" })}
          >
            <X size={18} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-5">
          <label className="form-control w-full">
            <span className="label pb-1">
              <span className="label-text font-medium">
                {t("manage.repositories.createNameLabel")}
              </span>
            </span>
            <input
              type="text"
              className="input input-bordered w-full"
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t("manage.repositories.createNamePlaceholder")}
              disabled={createMutation.isPending}
              autoFocus
              required
            />
          </label>

          <div className="space-y-2">
            <span className="text-sm font-medium">{t("manage.repositories.sourceLabel")}</span>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                className={`btn btn-sm ${source === "local" ? "btn-primary" : "btn-outline"}`}
                onClick={() => setSource("local")}
                disabled={createMutation.isPending}
              >
                {t("manage.repositories.sourceLocal")}
              </button>
              <button
                type="button"
                className={`btn btn-sm gap-2 ${source === "cloud" ? "btn-primary" : "btn-outline"}`}
                onClick={() => setSource("cloud")}
                disabled={createMutation.isPending}
              >
                <Cloud size={15} />
                {t("manage.repositories.sourceCloud")}
              </button>
            </div>
          </div>

          {source === "cloud" && (
            <div className="form-control w-full">
              <label className="label pb-1" htmlFor="repository-cloud-credential">
                <span className="label-text font-medium">
                  {t("manage.repositories.cloudCredentialLabel")}
                </span>
              </label>
              <select
                id="repository-cloud-credential"
                className="select select-bordered w-full"
                value={credentialId}
                onChange={(event) => setCredentialId(event.target.value)}
                disabled={createMutation.isPending || credentialsQuery.isLoading}
                required
              >
                <option value="">
                  {credentials.length === 0
                    ? t("manage.repositories.noCloudCredentials")
                    : t("manage.repositories.selectCloudCredential")}
                </option>
                {credentials.map((credential) => (
                  <option key={credential.id} value={credential.id}>
                    {credential.display_name} · {credential.provider_title ?? credential.provider} ·{" "}
                    {credential.masked_identity}
                  </option>
                ))}
              </select>
              <p className="mt-2 text-xs leading-relaxed text-base-content/60">
                {t("manage.repositories.cloudCredentialsHintPrefix")}{" "}
                <Link
                  to="/settings?tab=cloud"
                  className="link link-primary font-medium"
                  onClick={handleClose}
                >
                  {t("manage.repositories.cloudCredentialsHintLink")}
                </Link>
                {t("manage.repositories.cloudCredentialsHintSuffix")}
              </p>
            </div>
          )}

          <div className="modal-action">
            <button
              type="button"
              className="btn btn-ghost"
              onClick={handleClose}
              disabled={createMutation.isPending}
            >
              {t("common.cancel", { defaultValue: "Cancel" })}
            </button>
            <button
              type="submit"
              className="btn btn-primary gap-2"
              disabled={
                !name.trim() ||
                createMutation.isPending ||
                (source === "cloud" && !credentialId)
              }
            >
              {createMutation.isPending ? (
                <span className="loading loading-spinner loading-xs" />
              ) : (
                <FolderPlus size={16} />
              )}
              {t("manage.repositories.createSubmit")}
            </button>
          </div>
        </form>
      </div>
      <button
        type="button"
        className="modal-backdrop"
        onClick={handleClose}
        aria-label={t("common.close", { defaultValue: "Close" })}
      />
    </div>
  );
}

export default function RepositoryGrid() {
  const { t } = useI18n();
  const showMessage = useMessage();
  const repositoriesQuery = useIndexingRepositories();
  const repositories = repositoriesQuery.repositories;
  const {
    scanRepository,
    scanRepositories,
    scanningIds,
    detectStacks,
    detectingIds,
    isScanning,
  } = useRepositoryScan();
  const detectDuplicatesMutation = useDetectDuplicates();
  const duplicateScanningRepositoryId =
    detectDuplicatesMutation.isPending && detectDuplicatesMutation.variables
      ? detectDuplicatesMutation.variables.repositoryId
      : undefined;
  const locationRebuildMutation = $api.useMutation(
    "post",
    "/api/v1/locations/rebuild",
  );
  const cloudImportMutation = useStartRepositoryCloudImport();
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  const repositoryIds = useMemo(
    () => repositories.map((repository) => repository.id).filter(Boolean),
    [repositories],
  );

  const handleScanRepository = useCallback(
    async (repository: IndexingRepositoryOption) => {
      try {
        await scanRepository(repository.id);
        showMessage(
          "success",
          t("manage.repositories.scanQueued", {
            name: getRepositoryDisplayName(repository, t),
          }),
        );
      } catch (error) {
        showMessage(
          "error",
          error instanceof Error
            ? error.message
            : t("manage.repositories.scanFailed"),
        );
      }
    },
    [scanRepository, showMessage, t],
  );

  const handleDetectStacks = useCallback(
    async (repository: IndexingRepositoryOption) => {
      try {
        const created = await detectStacks(repository.id);
        showMessage(
          "success",
          t("manage.repositories.detectStacksCompleted", {
            name: getRepositoryDisplayName(repository, t),
            count: created,
          }),
        );
      } catch (error) {
        showMessage(
          "error",
          error instanceof Error
            ? error.message
            : t("manage.repositories.detectStacksFailed"),
        );
      }
    },
    [detectStacks, showMessage, t],
  );

  const handleDuplicateScan = useCallback(
    async (repository: IndexingRepositoryOption) => {
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
            message:
              error instanceof Error
                ? error.message
                : t("manage.repositories.duplicateScanFailed"),
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
    async (repository: IndexingRepositoryOption) => {
      try {
        await locationRebuildMutation.mutateAsync({
          body: { repository_id: repository.id },
        });
        showMessage("success", t("manage.repositories.rebuildLocationQueued"));
      } catch (error) {
        showMessage(
          "error",
          error instanceof Error ? error.message : String(error),
        );
      }
    },
    [locationRebuildMutation, showMessage, t],
  );

  const cloudImportingRepositoryId =
    cloudImportMutation.isPending && cloudImportMutation.variables
      ? (cloudImportMutation.variables as { params?: { path?: { id?: string } } }).params?.path?.id
      : undefined;

  const handleCloudImport = useCallback(
    async (repository: IndexingRepositoryOption) => {
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
            name: getRepositoryDisplayName(repository, t),
          }),
        );
      } catch (error) {
        showMessage(
          "error",
          error instanceof Error ? error.message : t("manage.repositories.cloudImportFailed"),
        );
      }
    },
    [cloudImportMutation, showMessage, t],
  );

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
      showMessage(
        "error",
        error instanceof Error
          ? error.message
          : t("manage.repositories.scanFailed"),
      );
    }
  }, [repositoryIds, scanRepositories, showMessage, t]);

  return (
    <section className="container mx-auto max-w-5xl px-4 pb-12">
      <div className="mb-4 flex items-center justify-between gap-4">
        <div>
          <h2 className="text-base font-semibold">
            {t("manage.repositories.title")}
          </h2>
          <p className="text-sm text-base-content/60">
            {t("manage.repositories.description")}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            className="btn btn-sm btn-soft btn-primary gap-2"
            onClick={() => setIsCreateOpen(true)}
            title={t("manage.repositories.createAction")}
          >
            <Plus size={16} />
            <span className="hidden sm:inline">
              {t("manage.repositories.createAction")}
            </span>
          </button>
          <button
            type="button"
            className="btn btn-sm btn-soft btn-info gap-2"
            onClick={handleScanAll}
            disabled={repositoryIds.length === 0 || isScanning}
            title={t("manage.repositories.scanAll")}
          >
            {isScanning ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <RefreshCcwDot size={16} />
            )}
            <span className="hidden sm:inline">
              {t("manage.repositories.scanAll")}
            </span>
          </button>
        </div>
      </div>

      {repositoriesQuery.isLoading ? (
        <div className="flex h-32 items-center justify-center rounded-lg border border-base-300">
          <span className="loading loading-spinner loading-md" />
        </div>
      ) : repositoriesQuery.isError ? (
        <div className="rounded-lg border border-error/30 bg-error/10 px-4 py-3 text-sm text-error">
          {t("manage.repositories.unavailable")}
        </div>
      ) : repositories.length === 0 ? (
        <div className="rounded-lg border border-base-300 px-4 py-8 text-center text-sm text-base-content/60">
          {t("manage.repositories.empty")}
        </div>
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {repositories.map((repository) => (
            <RepositoryCard
              key={repository.id}
              repository={repository}
              isScanning={scanningIds.has(repository.id)}
              isDetecting={detectingIds.has(repository.id)}
              isDuplicateScanning={
                duplicateScanningRepositoryId === repository.id
              }
              isRebuildingLocation={rebuildingLocationId === repository.id}
              isCloudImporting={cloudImportingRepositoryId === repository.id}
              onScan={handleScanRepository}
              onDetectStacks={handleDetectStacks}
              onDuplicateScan={handleDuplicateScan}
              onLocationRebuild={handleLocationRebuild}
              onCloudImport={handleCloudImport}
            />
          ))}
        </div>
      )}

      <AddRepositoryModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
      />
    </section>
  );
}
