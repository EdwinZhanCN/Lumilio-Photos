import { useState, type KeyboardEvent } from "react";
import {
  ChevronDown,
  CircleStop,
  Cloud,
  CloudDownload,
  Copy,
  Ellipsis,
  FolderSearch,
  Layers,
  MapPin,
  Pencil,
  RefreshCcw,
  Trash2,
} from "lucide-react";
import { CloudSourcesModal, useRepositoryCloudStatus } from "@/features/cloud";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import type { RepositoryOption } from "../../types";
import { useRepositoryAssetCount } from "../../api/useRepositoryAssetCount";
import { getStorageEntityDisplayName } from "../../model/storageEntities";
import { getRepositoryEffectiveState } from "../../model/repositoryOptions";
import RemoveRepositoryModal from "./RemoveRepositoryModal";
import RenameRepositoryModal from "./RenameRepositoryModal";

export interface RepositoryRowProps {
  repository: RepositoryOption;
  rootStatus?: string;
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
  onLocate?: (repository: RepositoryOption) => void;
}

export default function RepositoryRow({
  repository,
  rootStatus,
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
  onLocate,
}: RepositoryRowProps) {
  const { t } = useI18n();
  const countQuery = useRepositoryAssetCount(repository.id);
  const cloudStatusQuery = useRepositoryCloudStatus(repository.id);
  const scanQuery = $api.useQuery(
    "get",
    "/api/v1/repositories/{id}/scans/latest",
    { params: { path: { id: repository.id } } },
    {
      enabled: Boolean(repository.id),
      retry: false,
      staleTime: 1_000,
      refetchInterval: (query) => {
        const status = query.state.data?.status;
        return isActiveScanStatus(status) ? 1_000 : false;
      },
    },
  );
  const cancelScanMutation = $api.useMutation(
    "post",
    "/api/v1/repositories/{id}/scans/{operation_id}/cancel",
  );
  const latestScan = scanQuery.data;
  const isBackgroundScanning = isActiveScanStatus(latestScan?.status);
  const cloudStatus = cloudStatusQuery.data;
  const latestRun = cloudStatus?.latest_run;
  const name = getStorageEntityDisplayName(repository, t);
  const [expanded, setExpanded] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);
  const [removeOpen, setRemoveOpen] = useState(false);
  const [cloudSourcesOpen, setCloudSourcesOpen] = useState(false);
  const isBusy =
    isScanning || isDetecting || isDuplicateScanning || isRebuildingLocation || isCloudImporting;
  const hasCloudBinding = Boolean(cloudStatus?.credential);
  const latestRunStatus = latestRun?.status;
  const effectiveState = getRepositoryEffectiveState(repository, rootStatus);
  const isUnavailable = effectiveState !== "active";
  const lifecycle = getLifecycleIndicator(effectiveState, t);

  const toggleExpanded = () => setExpanded((current) => !current);
  const cancelActiveScan = async () => {
    if (!latestScan?.operation_id) return;
    await cancelScanMutation.mutateAsync({
      params: {
        path: {
          id: repository.id,
          operation_id: latestScan.operation_id,
        },
      },
    });
    await scanQuery.refetch();
  };
  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) return;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      toggleExpanded();
    }
  };

  return (
    <li>
      <div
        className="cursor-pointer px-3 py-2.5"
        role="button"
        tabIndex={0}
        aria-expanded={expanded}
        onClick={toggleExpanded}
        onKeyDown={handleKeyDown}
      >
        <div className="flex items-center gap-3">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="truncate text-sm font-medium text-base-content">{name}</span>
              {hasCloudBinding && (
                <span className="badge badge-info badge-sm badge-soft">
                  {t("manage.repositories.sourceCloud")}
                </span>
              )}
              {isUnavailable && (
                <span className={`badge badge-sm badge-soft ${lifecycle.badgeClass}`}>
                  {lifecycle.label}
                </span>
              )}
            </div>
            <p className="mt-0.5 truncate text-xs text-base-content/55" title={repository.path}>
              {repository.path}
            </p>
          </div>
          {isBusy ? (
            <span
              className="loading loading-spinner loading-sm shrink-0 text-primary"
              aria-label={repositoryActivityLabel(repository.activity, t)}
            />
          ) : !isUnavailable && repository.activity !== "idle" ? (
            <span className="badge badge-info badge-sm shrink-0">
              {repositoryActivityLabel(repository.activity, t)}
            </span>
          ) : null}
          <span
            className="tooltip z-10 shrink-0 tabular-nums text-sm font-semibold"
            data-tip={t("manage.repositories.assetCount")}
          >
            {countQuery.isLoading ? (
              <span className="loading loading-dots loading-xs" />
            ) : (
              countQuery.assetCount.toLocaleString()
            )}
          </span>
          <span className="tooltip inline-flex shrink-0 items-center" data-tip={lifecycle.label}>
            <span className={`status status-md ${lifecycle.statusClass}`} aria-hidden="true" />
          </span>
          <div
            className="dropdown dropdown-top dropdown-end shrink-0"
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
          >
            <div
              tabIndex={0}
              role="button"
              className="btn btn-ghost btn-sm btn-square"
              aria-label={t("manage.repositories.actionsMenu", { name })}
            >
              <Ellipsis size={16} />
            </div>
            <ul
              tabIndex={-1}
              className="dropdown-content menu menu-sm z-dropdown mb-1 w-60 rounded-box border border-base-300 bg-base-100 shadow-xl"
            >
              <li className="menu-title">{t("manage.repositories.menuGroupManage", "Manage")}</li>
              {repository.role !== "primary" ? (
                <li>
                  <button
                    type="button"
                    onClick={() => setRenameOpen(true)}
                    disabled={isBusy || isUnavailable}
                  >
                    <Pencil size={15} />
                    {t("manage.repositories.rename", "Rename Repository")}
                  </button>
                </li>
              ) : null}
              <li>
                <button
                  type="button"
                  onClick={() => setCloudSourcesOpen(true)}
                  disabled={isBusy || isUnavailable}
                >
                  <Cloud size={15} />
                  {t("manage.repositories.manageCloudSources", "Manage cloud sources")}
                </button>
              </li>
              {isBackgroundScanning ? (
                <li>
                  <button
                    type="button"
                    className="text-warning"
                    onClick={() => void cancelActiveScan()}
                    disabled={
                      cancelScanMutation.isPending || Boolean(latestScan?.cancellation_requested)
                    }
                  >
                    {cancelScanMutation.isPending ? (
                      <span className="loading loading-spinner loading-xs" />
                    ) : (
                      <CircleStop size={15} />
                    )}
                    {latestScan?.cancellation_requested
                      ? t(
                          "manage.repositories.scanCancellationRequestedAction",
                          "Cancellation requested",
                        )
                      : t("manage.repositories.cancelScan", "Cancel verification")}
                  </button>
                </li>
              ) : null}
              {isUnavailable && onLocate ? (
                <li>
                  <button type="button" onClick={() => onLocate(repository)} disabled={isBusy}>
                    <FolderSearch size={15} />
                    {t("manage.repositories.hostAction.locateRepository", "Locate Repository")}
                  </button>
                </li>
              ) : null}
              <li className="menu-title">
                {t("manage.repositories.menuGroupMaintenance", "Maintenance")}
              </li>
              <li>
                <button
                  type="button"
                  onClick={() => onScan(repository)}
                  disabled={isScanning || isUnavailable}
                >
                  {isScanning || isBackgroundScanning ? (
                    <span className="loading loading-spinner loading-xs" />
                  ) : (
                    <RefreshCcw size={15} />
                  )}
                  {t("manage.repositories.rescanRepository", { name })}
                </button>
              </li>
              <li>
                <button
                  type="button"
                  onClick={() => onDetectStacks(repository)}
                  disabled={isBusy || isUnavailable}
                >
                  {isDetecting ? (
                    <span className="loading loading-spinner loading-xs" />
                  ) : (
                    <Layers size={15} />
                  )}
                  {t("manage.repositories.detectStacks", { name })}
                </button>
              </li>
              <li>
                <button
                  type="button"
                  onClick={() => onDuplicateScan(repository)}
                  disabled={isBusy || isUnavailable}
                >
                  {isDuplicateScanning ? (
                    <span className="loading loading-spinner loading-xs" />
                  ) : (
                    <Copy size={15} />
                  )}
                  {t("manage.repositories.duplicateScan")}
                </button>
              </li>
              <li>
                <button
                  type="button"
                  onClick={() => onLocationRebuild(repository)}
                  disabled={isBusy || isUnavailable}
                >
                  {isRebuildingLocation ? (
                    <span className="loading loading-spinner loading-xs" />
                  ) : (
                    <MapPin size={15} />
                  )}
                  {t("manage.repositories.rebuildLocation")}
                </button>
              </li>
              {hasCloudBinding && (
                <li>
                  <button
                    type="button"
                    onClick={() => onCloudImport(repository)}
                    disabled={
                      isBusy ||
                      isUnavailable ||
                      latestRunStatus === "running" ||
                      latestRunStatus === "queued"
                    }
                  >
                    {isCloudImporting ||
                    latestRunStatus === "running" ||
                    latestRunStatus === "queued" ? (
                      <span className="loading loading-spinner loading-xs" />
                    ) : (
                      <CloudDownload size={15} />
                    )}
                    {t("manage.repositories.importFromCloud")}
                  </button>
                </li>
              )}
              {repository.role !== "primary" && (
                <>
                  <li className="menu-title">
                    {t("manage.repositories.menuGroupDanger", "Danger zone")}
                  </li>
                  <li>
                    <button
                      type="button"
                      className="text-error"
                      onClick={() => setRemoveOpen(true)}
                      disabled={isBusy || repository.activity !== "idle"}
                    >
                      <Trash2 size={15} />
                      {t("manage.repositories.removeAction", "Remove from Lumilio")}
                    </button>
                  </li>
                </>
              )}
            </ul>
          </div>
          <ChevronDown
            size={16}
            className={`shrink-0 text-base-content/40 transition-transform ${expanded ? "rotate-180" : ""}`}
            aria-hidden="true"
          />
        </div>
      </div>

      {expanded ? (
        <div className="px-3 pb-3">
          <div className="space-y-3 rounded-lg border border-base-200 bg-base-200/30 p-3">
            {(latestScan?.status === "completed" || latestScan?.status === "partial") && (
              <div>
                <div className="flex flex-wrap items-center gap-2">
                  <span
                    className={`badge badge-sm badge-soft ${
                      latestScan.status === "completed" ? "badge-success" : "badge-warning"
                    }`}
                  >
                    {latestScan.status === "completed"
                      ? t("manage.repositories.scanAuthoritative", "Verification complete")
                      : t("manage.repositories.scanPartial", "Verification partial")}
                  </span>
                  {latestScan.partial_coverage && (
                    <span className="text-xs text-warning">
                      {t(
                        "manage.repositories.scanPartialHelp",
                        "Some directories could not be verified. Previously valid files remain available.",
                      )}
                    </span>
                  )}
                </div>
                <div className="stats mt-2 w-full border border-base-200 bg-base-100 shadow-none sm:stats-horizontal">
                  <ScanStat
                    label={t("manage.repositories.scanCountDirectories", "Directories")}
                    value={latestScan.directories_observed ?? 0}
                  />
                  <ScanStat
                    label={t("manage.repositories.scanCountFiles", "Files observed")}
                    value={latestScan.files_observed ?? 0}
                  />
                  <ScanStat
                    label={t("manage.repositories.scanBytesQueued", "Bytes queued")}
                    value={latestScan.bytes_queued ?? 0}
                  />
                  <ScanStat
                    label={t("manage.repositories.scanBytesHashed", "Bytes hashed")}
                    value={latestScan.bytes_hashed ?? 0}
                  />
                  <ScanStat
                    label={t("manage.repositories.scanOutboxDepth", "Pending work")}
                    value={latestScan.outbox_depth ?? 0}
                  />
                  <ScanStat
                    label={t("manage.repositories.scanDirectoryErrors", "Directory errors")}
                    value={latestScan.error_directories ?? 0}
                    tone={(latestScan.error_directories ?? 0) > 0 ? "text-warning" : undefined}
                  />
                </div>
              </div>
            )}
            {latestScan && isActiveScanStatus(latestScan.status) && (
              <p className="text-xs text-base-content/55">
                {latestScan.cancellation_requested
                  ? t(
                      "manage.repositories.scanCancellationRequested",
                      "Cancellation requested. The current bounded step will finish safely.",
                    )
                  : t(
                      "manage.repositories.scanStatusRunning",
                      "Scan phase: {{phase}}. You can keep using Lumilio while background work runs.",
                      { phase: scanPhaseLabel(latestScan.status, t) },
                    )}
              </p>
            )}
            {cancelScanMutation.isError && (
              <p className="text-xs text-error">
                {t(
                  "manage.repositories.cancelScanFailed",
                  "Could not request verification cancellation.",
                )}
              </p>
            )}
            {latestScan?.status === "failed" && (
              <p className="text-xs text-error">
                {t(
                  "manage.repositories.scanStatusFailed",
                  "Verification stopped after a stable error. Previously valid files remain available.",
                )}
              </p>
            )}
            {latestScan?.status === "cancelled" && (
              <p className="text-xs text-base-content/55">
                {t(
                  "manage.repositories.scanStatusCancelled",
                  "Verification was cancelled. Previously valid files remain available.",
                )}
              </p>
            )}
            {hasCloudBinding && latestRun && (
              <div className="flex flex-wrap items-center gap-2 text-sm">
                <Cloud size={14} className="text-base-content/55" />
                <span className="font-medium capitalize">{latestRun.status}</span>
                <span className="text-xs text-base-content/55">
                  {t(
                    "manage.repositories.cloudRunSummary",
                    "{{imported}} imported · {{failed}} failed",
                    {
                      imported: (latestRun.imported_count ?? 0).toLocaleString(),
                      failed: (latestRun.failed_count ?? 0).toLocaleString(),
                    },
                  )}
                </span>
              </div>
            )}
          </div>
        </div>
      ) : null}

      {repository.role !== "primary" ? (
        <RenameRepositoryModal
          repository={repository}
          isOpen={renameOpen}
          onClose={() => setRenameOpen(false)}
        />
      ) : null}
      <RemoveRepositoryModal
        repository={repository}
        isOpen={removeOpen}
        onClose={() => setRemoveOpen(false)}
      />
      <CloudSourcesModal
        repositoryId={repository.id}
        repositoryName={name}
        isOpen={cloudSourcesOpen}
        onClose={() => setCloudSourcesOpen(false)}
      />
    </li>
  );
}

type ActiveScanStatus = "queued" | "crawling" | "catching_up" | "finalizing";

function isActiveScanStatus(status?: string): status is ActiveScanStatus {
  return (
    status === "queued" ||
    status === "crawling" ||
    status === "catching_up" ||
    status === "finalizing"
  );
}

function scanPhaseLabel(status: ActiveScanStatus, t: ReturnType<typeof useI18n>["t"]): string {
  switch (status) {
    case "crawling":
      return t("manage.repositories.scanPhaseCrawling", "Observing files");
    case "catching_up":
      return t("manage.repositories.scanPhaseCatchingUp", "Catching up with changes");
    case "finalizing":
      return t("manage.repositories.scanPhaseFinalizing", "Finalizing verification");
    default:
      return t("manage.repositories.scanPhaseQueued", "Queued");
  }
}

function ScanStat({ label, value, tone }: { label: string; value: number; tone?: string }) {
  return (
    <div className="stat px-3 py-2">
      <div className="stat-title text-xs text-base-content/55">{label}</div>
      <div className={`stat-value text-base tabular-nums ${tone ?? ""}`}>
        {value.toLocaleString()}
      </div>
    </div>
  );
}

function repositoryActivityLabel(
  activity: RepositoryOption["activity"],
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (activity) {
    case "scanning":
      return t("manage.repositories.activityScanning", "Scanning");
    case "importing":
      return t("manage.repositories.activityImporting", "Importing");
    case "processing":
      return t("manage.repositories.activityProcessing", "Processing");
    case "paused":
      return t("manage.repositories.activityPaused", "Paused");
    default:
      return t("manage.repositories.activityIdle", "Idle");
  }
}

function getLifecycleIndicator(
  state: ReturnType<typeof getRepositoryEffectiveState>,
  t: ReturnType<typeof useI18n>["t"],
): { statusClass: string; badgeClass: string; label: string } {
  switch (state) {
    case "active":
      return {
        statusClass: "status-success",
        badgeClass: "badge-success",
        label: t("manage.repositories.lifecycleActive", "Active"),
      };
    case "storage_location_offline":
      return {
        statusClass: "status-warning",
        badgeClass: "badge-warning",
        label: t("manage.repositories.storageLocationOfflineBadge", "Storage Location offline"),
      };
    case "storage_location_error":
      return {
        statusClass: "status-error",
        badgeClass: "badge-error",
        label: t(
          "manage.repositories.storageLocationErrorBadge",
          "Storage Location needs attention",
        ),
      };
    case "storage_location_maintenance":
      return {
        statusClass: "status-info",
        badgeClass: "badge-info",
        label: t(
          "manage.repositories.storageLocationMaintenanceBadge",
          "Storage Location maintenance",
        ),
      };
    case "identity_error":
      return {
        statusClass: "status-error",
        badgeClass: "badge-error",
        label: t("manage.repositories.identityErrorBadge", "Repository identity mismatch"),
      };
    case "recovery_required":
      return {
        statusClass: "status-error",
        badgeClass: "badge-error",
        label: t("manage.repositories.recoveryRequiredBadge", "Recovery required"),
      };
    case "maintenance":
      return {
        statusClass: "status-info",
        badgeClass: "badge-info",
        label: t("manage.repositories.maintenanceBadge", "Maintenance"),
      };
    default:
      return {
        statusClass: "status-warning",
        badgeClass: "badge-warning",
        label: t("manage.repositories.offlineBadge", "Offline"),
      };
  }
}
