import { useState } from "react";
import { Plus, RefreshCcwDot, ScanFace } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import type { RepositoryOption } from "../../types";
import AddRepositoryModal from "./AddRepositoryModal";
import RepositoryCard from "./RepositoryCard";

export interface RepositoryGridProps {
  repositories: RepositoryOption[];
  repositoryIds: string[];
  isLoading: boolean;
  isError: boolean;
  isScanning: boolean;
  isRebuildingPeople: boolean;
  scanningIds: Set<string>;
  detectingIds: Set<string>;
  duplicateScanningRepositoryId?: string;
  rebuildingLocationId: string | null;
  cloudImportingRepositoryId?: string;
  onScanRepository: (repository: RepositoryOption) => void;
  onDetectStacks: (repository: RepositoryOption) => void;
  onDuplicateScan: (repository: RepositoryOption) => void;
  onLocationRebuild: (repository: RepositoryOption) => void;
  onCloudImport: (repository: RepositoryOption) => void;
  onScanAll: () => void;
  onRebuildPeople: () => void;
}

export default function RepositoryGrid({
  repositories,
  repositoryIds,
  isLoading,
  isError,
  isScanning,
  isRebuildingPeople,
  scanningIds,
  detectingIds,
  duplicateScanningRepositoryId,
  rebuildingLocationId,
  cloudImportingRepositoryId,
  onScanRepository,
  onDetectStacks,
  onDuplicateScan,
  onLocationRebuild,
  onCloudImport,
  onScanAll,
  onRebuildPeople,
}: RepositoryGridProps) {
  const { t } = useI18n();
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  return (
    <section className="container mx-auto max-w-5xl px-4 pb-12">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-4">
        <div className="min-w-0">
          <h2 className="text-base font-semibold">{t("manage.repositories.title")}</h2>
          <p className="text-sm text-base-content/60">{t("manage.repositories.description")}</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            className="btn btn-sm btn-soft btn-primary gap-2"
            onClick={() => setIsCreateOpen(true)}
            title={t("manage.repositories.createAction")}
          >
            <Plus size={16} />
            <span className="hidden sm:inline">{t("manage.repositories.createAction")}</span>
          </button>
          <button
            type="button"
            className="btn btn-sm btn-soft btn-info gap-2"
            onClick={onScanAll}
            disabled={repositoryIds.length === 0 || isScanning}
            title={t("manage.repositories.scanAll")}
          >
            {isScanning ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <RefreshCcwDot size={16} />
            )}
            <span className="hidden sm:inline">{t("manage.repositories.scanAll")}</span>
          </button>
          <button
            type="button"
            className="btn btn-sm btn-soft gap-2"
            onClick={onRebuildPeople}
            disabled={isRebuildingPeople}
            title={t("people.rebuild.action")}
          >
            {isRebuildingPeople ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <ScanFace size={16} />
            )}
            <span className="hidden sm:inline">
              {isRebuildingPeople ? t("people.rebuild.running") : t("people.rebuild.action")}
            </span>
          </button>
        </div>
      </div>

      {isLoading ? (
        <div className="flex h-32 items-center justify-center rounded-lg border border-base-300">
          <span className="loading loading-spinner loading-md" />
        </div>
      ) : isError ? (
        <div className="rounded-lg border border-error/30 bg-error/10 px-4 py-3 text-sm text-error">
          {t("manage.repositories.unavailable")}
        </div>
      ) : repositories.length === 0 ? (
        <div className="rounded-lg border border-base-300 px-4 py-8 text-center text-sm text-base-content/60">
          {t("manage.repositories.empty")}
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {repositories.map((repository) => (
            <RepositoryCard
              key={repository.id}
              repository={repository}
              isScanning={scanningIds.has(repository.id)}
              isDetecting={detectingIds.has(repository.id)}
              isDuplicateScanning={duplicateScanningRepositoryId === repository.id}
              isRebuildingLocation={rebuildingLocationId === repository.id}
              isCloudImporting={cloudImportingRepositoryId === repository.id}
              onScan={onScanRepository}
              onDetectStacks={onDetectStacks}
              onDuplicateScan={onDuplicateScan}
              onLocationRebuild={onLocationRebuild}
              onCloudImport={onCloudImport}
            />
          ))}
        </div>
      )}

      <AddRepositoryModal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} />
    </section>
  );
}
