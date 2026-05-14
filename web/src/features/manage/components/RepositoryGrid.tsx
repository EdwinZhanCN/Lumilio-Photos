import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Copy,
  Ellipsis,
  Folder,
  FolderPlus,
  Layers,
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

type AssetListResponse = {
  data?: {
    total?: number;
  };
};

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
    assetCount: ((query.data as AssetListResponse | undefined)?.data?.total ??
      0) as number,
  };
}

function RepositoryCard({
  repository,
  isScanning,
  isDetecting,
  isDuplicateScanning,
  onScan,
  onDetectStacks,
  onDuplicateScan,
}: {
  repository: IndexingRepositoryOption;
  isScanning: boolean;
  isDetecting: boolean;
  isDuplicateScanning: boolean;
  onScan: (repository: IndexingRepositoryOption) => void;
  onDetectStacks: (repository: IndexingRepositoryOption) => void;
  onDuplicateScan: (repository: IndexingRepositoryOption) => void;
}) {
  const { t } = useI18n();
  const countQuery = useRepositoryAssetCount(repository.id);
  const name = getRepositoryDisplayName(repository, t);
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const isBusy = isScanning || isDetecting || isDuplicateScanning;

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
            </div>
            <p className="mt-1 truncate text-xs text-base-content/55" title={repository.path}>
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
  const [name, setName] = useState("");

  const handleClose = useCallback(() => {
    if (createMutation.isPending) return;
    setName("");
    onClose();
  }, [createMutation.isPending, onClose]);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      const trimmedName = name.trim();
      if (!trimmedName || createMutation.isPending) return;

      try {
        await createMutation.mutateAsync({
          body: {
            name: trimmedName,
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
          "success",
          t("manage.repositories.createSuccess", { name: trimmedName }),
        );
        setName("");
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
    [createMutation, name, onClose, queryClient, showMessage, t],
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
              <p className="text-sm text-base-content/60">
                {t("manage.repositories.createDescription")}
              </p>
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
            <span className="label pt-2">
              <span className="label-text-alt text-base-content/55">
                {t("manage.repositories.createNameHint")}
              </span>
            </span>
          </label>

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
              disabled={!name.trim() || createMutation.isPending}
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
  } =
    useRepositoryScan();
  const detectDuplicatesMutation = useDetectDuplicates();
  const duplicateScanningRepositoryId =
    detectDuplicatesMutation.isPending &&
    detectDuplicatesMutation.variables
      ? detectDuplicatesMutation.variables.repositoryId
      : undefined;
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
              onScan={handleScanRepository}
              onDetectStacks={handleDetectStacks}
              onDuplicateScan={handleDuplicateScan}
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
