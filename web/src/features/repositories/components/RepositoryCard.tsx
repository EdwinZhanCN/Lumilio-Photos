import { useEffect, useRef, useState } from "react";
import {
  Cloud,
  CloudDownload,
  Copy,
  Ellipsis,
  Folder,
  Layers,
  MapPin,
  RefreshCcw,
} from "lucide-react";
import { useRepositoryCloudStatus } from "@/features/cloud";
import { useI18n } from "@/lib/i18n";
import type { RepositoryOption } from "../types";
import { useRepositoryAssetCount } from "../api/useRepositoryAssetCount";
import { getRepositoryDisplayName } from "../utils/repositoryDisplayName";

export interface RepositoryCardProps {
  repository: RepositoryOption;
  isScanning: boolean;
  isDetecting: boolean;
  isDuplicateScanning: boolean;
  isRebuildingLocation: boolean;
  isCloudImporting: boolean;
  onScan: (repository: RepositoryOption) => void;
  onDetectStacks: (repository: RepositoryOption) => void;
  onDuplicateScan: (repository: RepositoryOption) => void;
  onLocationRebuild: (repository: RepositoryOption) => void;
  onCloudImport: (repository: RepositoryOption) => void;
}

export default function RepositoryCard({
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
}: RepositoryCardProps) {
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
              <h3 className="truncate text-sm font-semibold text-base-content">{name}</h3>
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
                  {isCloudImporting ||
                  latestRunStatus === "running" ||
                  latestRunStatus === "queued" ? (
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
          <div className="text-xs text-base-content/55">{t("manage.repositories.assetCount")}</div>
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
