import { useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Activity,
  ChevronDown,
  FolderOpen,
  HardDrive,
  Plus,
  RefreshCcwDot,
  ScanFace,
  TriangleAlert,
  Wrench,
} from "lucide-react";
import { formatBytes } from "@/lib/utils/formatters";
import type { components } from "@/lib/http-commons/schema";
import { useI18n } from "@/lib/i18n";
import type { RepositoryOption } from "../../types";
import { useRepositoryRoots } from "../../api/useRepositoryRoots";
import {
  type HostActionKind,
  useNativeHostCapability,
  useUnfinishedNativeHostActions,
} from "../../api/useNativeHostActions";
import AddRepositoryModal from "./AddRepositoryModal";
import NativeHostActionModal from "./NativeHostActionModal";
import RemoveStorageLocationModal from "./RemoveStorageLocationModal";
import RepositoryCandidateModal from "./RepositoryCandidateModal";
import RepositoryRow from "./RepositoryRow";

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

type RepositoryRoot = components["schemas"]["dto.RepositoryRootDTO"];

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
  const navigate = useNavigate();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isCandidateOpen, setIsCandidateOpen] = useState(false);
  const [removeRootTarget, setRemoveRootTarget] = useState<RepositoryRoot | null>(null);
  const [collapsedRootIds, setCollapsedRootIds] = useState<ReadonlySet<string>>(new Set());
  const [hostAction, setHostAction] = useState<{
    kind: HostActionKind;
    rootId?: string;
    repositoryId?: string;
  } | null>(null);
  const rootsQuery = useRepositoryRoots();
  const nativeHostQuery = useNativeHostCapability();
  const nativeHostAvailable = nativeHostQuery.data?.available === true;
  const unfinishedHostActions = useUnfinishedNativeHostActions(nativeHostAvailable);

  const groups = useMemo(() => {
    const roots = rootsQuery.data?.roots ?? [];
    const rootIds = new Set(roots.map((root) => root.id ?? ""));
    const reposByRoot = new Map<string, RepositoryOption[]>();
    const unlinked: RepositoryOption[] = [];
    for (const repository of repositories) {
      if (rootIds.has(repository.rootId)) {
        const list = reposByRoot.get(repository.rootId) ?? [];
        list.push(repository);
        reposByRoot.set(repository.rootId, list);
      } else {
        unlinked.push(repository);
      }
    }
    return { roots, reposByRoot, unlinked };
  }, [repositories, rootsQuery.data?.roots]);

  const rootById = useMemo(
    () => new Map(groups.roots.map((root) => [root.id ?? "", root])),
    [groups.roots],
  );

  const toggleRootCollapsed = (rootId: string) => {
    setCollapsedRootIds((current) => {
      const next = new Set(current);
      if (next.has(rootId)) next.delete(rootId);
      else next.add(rootId);
      return next;
    });
  };

  const openExistingRepository = () => {
    if (nativeHostAvailable) setHostAction({ kind: "open_repository" });
    else setIsCandidateOpen(true);
  };

  const renderRow = (repository: RepositoryOption) => (
    <RepositoryRow
      key={repository.id}
      repository={repository}
      rootStatus={rootById.get(repository.rootId)?.status}
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
      onLocate={
        nativeHostAvailable &&
        !repository.isPrimary &&
        rootById.get(repository.rootId)?.status === "active"
          ? () =>
              setHostAction({
                kind: "locate_repository",
                repositoryId: repository.id,
              })
          : undefined
      }
    />
  );

  return (
    <section className="container mx-auto max-w-5xl px-4 pb-12">
      {(unfinishedHostActions.data?.length ?? 0) > 0 ? (
        <button
          type="button"
          className="alert alert-info mb-4 w-full text-left"
          onClick={() => {
            const action = unfinishedHostActions.data?.[0];
            if (action && isHostActionKind(action.kind)) {
              setHostAction({
                kind: action.kind,
                rootId: action.root_id,
                repositoryId: action.repository_id,
              });
            }
          }}
        >
          <Activity className="size-5" />
          <span>
            {t(
              "manage.repositories.hostAction.unfinished",
              "You have {{count}} unfinished storage request. Resume it here.",
              { count: unfinishedHostActions.data?.length ?? 0 },
            )}
          </span>
        </button>
      ) : null}

      <div className="mb-4 flex flex-wrap items-center justify-between gap-4">
        <div className="min-w-0">
          <h2 className="text-base font-semibold">
            {t("manage.repositories.title", "Repositories")}
          </h2>
          <p className="text-sm text-base-content/60">{t("manage.repositories.description")}</p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <div className="join">
            <button
              type="button"
              className="btn btn-primary btn-sm join-item gap-2"
              onClick={() => setIsCreateOpen(true)}
            >
              <Plus size={16} />
              <span>{t("manage.repositories.createAction")}</span>
            </button>
            <div className="dropdown dropdown-end">
              <div
                tabIndex={0}
                role="button"
                className="btn btn-primary btn-sm join-item px-2"
                aria-label={t(
                  "manage.repositories.moreCreateOptions",
                  "More ways to add repositories",
                )}
              >
                <ChevronDown size={16} />
              </div>
              <ul
                tabIndex={-1}
                className="dropdown-content menu menu-sm z-dropdown mt-2 w-64 rounded-box border border-base-300 bg-base-100 shadow-xl"
              >
                <li>
                  <button type="button" onClick={openExistingRepository}>
                    <FolderOpen size={15} />
                    {t("manage.repositories.hostAction.openRepository", "Open Existing Repository")}
                  </button>
                </li>
                {nativeHostAvailable ? (
                  <li>
                    <button
                      type="button"
                      onClick={() => setHostAction({ kind: "authorize_storage_location" })}
                    >
                      <HardDrive size={15} />
                      {t("manage.repositories.hostAction.addLocation", "Add Storage Location")}
                    </button>
                  </li>
                ) : null}
              </ul>
            </div>
          </div>
          <div className="dropdown dropdown-end">
            <div
              tabIndex={0}
              role="button"
              className="btn btn-soft btn-sm gap-2"
              aria-label={t("manage.repositories.maintenance", "Maintenance")}
            >
              <Wrench size={16} />
              <span>{t("manage.repositories.maintenance", "Maintenance")}</span>
              <ChevronDown size={14} />
            </div>
            <ul
              tabIndex={-1}
              className="dropdown-content menu menu-sm z-dropdown mt-2 w-64 rounded-box border border-base-300 bg-base-100 shadow-xl"
            >
              <li>
                <button
                  type="button"
                  onClick={onScanAll}
                  disabled={repositoryIds.length === 0 || isScanning}
                >
                  {isScanning ? (
                    <span className="loading loading-spinner loading-xs" />
                  ) : (
                    <RefreshCcwDot size={15} />
                  )}
                  {t("manage.repositories.scanAll")}
                </button>
              </li>
              <li>
                <button type="button" onClick={onRebuildPeople} disabled={isRebuildingPeople}>
                  {isRebuildingPeople ? (
                    <span className="loading loading-spinner loading-xs" />
                  ) : (
                    <ScanFace size={15} />
                  )}
                  {t("people.rebuild.action", "Rebuild Person Recognition")}
                </button>
              </li>
              <li>
                <button type="button" onClick={() => void navigate("/server-monitor?tab=storage")}>
                  <Activity size={15} />
                  {t("manage.repositories.storageMonitor", "Open storage monitor")}
                </button>
              </li>
            </ul>
          </div>
        </div>
      </div>

      {isLoading ? (
        <div className="card border border-base-300 bg-base-100 p-4 shadow-sm">
          <div className="space-y-3">
            <div className="skeleton h-5 w-40" />
            <div className="skeleton h-10 w-full" />
            <div className="skeleton h-10 w-full" />
            <div className="skeleton h-10 w-full" />
          </div>
        </div>
      ) : isError ? (
        <div role="alert" className="alert alert-error alert-soft">
          {t("manage.repositories.unavailable")}
        </div>
      ) : repositories.length === 0 && groups.roots.length === 0 ? (
        <div className="rounded-lg border border-base-300 px-4 py-8 text-center text-sm text-base-content/60">
          {t("manage.repositories.empty")}
        </div>
      ) : (
        <div className="space-y-4">
          {groups.roots.map((root) => {
            const rootRepos = groups.reposByRoot.get(root.id ?? "") ?? [];
            const collapsed = collapsedRootIds.has(root.id ?? "");
            const removeBlockedLabel = storageRootRemovalBlockedLabel(root.removal_blocked_by, t);
            return (
              <section key={root.id} className="card border border-base-300 bg-base-100 shadow-sm">
                <header className="flex flex-wrap items-center gap-3 px-3 py-3">
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs btn-square"
                    onClick={() => toggleRootCollapsed(root.id ?? "")}
                    aria-expanded={!collapsed}
                    aria-label={t(
                      "manage.repositories.toggleLocationSection",
                      'Toggle the "{{name}}" section',
                      { name: root.name ?? "" },
                    )}
                  >
                    <ChevronDown
                      size={16}
                      className={`transition-transform ${collapsed ? "-rotate-90" : ""}`}
                    />
                  </button>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-sm font-semibold">{root.name}</span>
                      {root.kind === "default" ? (
                        <span className="badge badge-primary badge-sm badge-soft">
                          {t("manage.repositories.storageLocationDefault", "Default")}
                        </span>
                      ) : null}
                      {root.status !== "active" ? (
                        <span className="badge badge-warning badge-sm badge-soft">
                          {storageRootStatusLabel(root.status, t)}
                        </span>
                      ) : null}
                      {root.writable ? null : (
                        <span className="badge badge-ghost badge-sm">
                          {t("manage.repositories.storageLocationReadOnly", "Read-only")}
                        </span>
                      )}
                    </div>
                    <div className="mt-0.5 truncate text-xs text-base-content/55" title={root.path}>
                      {root.path}
                      {root.capacity_known
                        ? ` · ${t(
                            "manage.repositories.storageLocationCapacity",
                            "{{available}} available of {{total}}",
                            {
                              available: formatBytes(root.available_bytes ?? 0),
                              total: formatBytes(root.total_bytes ?? 0),
                            },
                          )}`
                        : ` · ${t(
                            "manage.repositories.storageLocationCapacityUnknown",
                            "Capacity unavailable",
                          )}`}
                      {` · ${t(
                        "manage.repositories.storageLocationRepositoryCount",
                        "{{count}} repositories",
                        { count: root.repository_count ?? 0 },
                      )}`}
                    </div>
                  </div>
                  <div className="flex shrink-0 items-center gap-2">
                    <span
                      className="tooltip inline-flex shrink-0 items-center"
                      data-tip={storageRootStatusLabel(root.status, t)}
                    >
                      <span
                        className={`status status-md ${rootStatusDot(root.status)}`}
                        aria-hidden="true"
                      />
                    </span>
                    {nativeHostAvailable && root.kind === "external" && root.status !== "active" ? (
                      <button
                        type="button"
                        className="btn btn-sm btn-soft"
                        onClick={() =>
                          setHostAction({ kind: "locate_storage_location", rootId: root.id })
                        }
                      >
                        <FolderOpen size={15} />
                        {t(
                          "manage.repositories.hostAction.locateLocation",
                          "Reconnect Storage Location",
                        )}
                      </button>
                    ) : null}
                    {root.kind === "external" ? (
                      <span
                        className={removeBlockedLabel ? "tooltip" : undefined}
                        data-tip={removeBlockedLabel || undefined}
                      >
                        <button
                          type="button"
                          className="btn btn-sm btn-ghost text-warning"
                          disabled={!root.can_remove}
                          onClick={() => setRemoveRootTarget(root)}
                        >
                          {t("manage.repositories.storageLocationRemove", "Remove")}
                        </button>
                      </span>
                    ) : null}
                  </div>
                </header>
                {collapsed ? null : rootRepos.length > 0 ? (
                  <ul className="list border-t border-base-200">{rootRepos.map(renderRow)}</ul>
                ) : (
                  <p className="border-t border-base-200 px-4 py-5 text-center text-xs text-base-content/50">
                    {t(
                      "manage.repositories.locationEmpty",
                      "No repositories in this Storage Location yet.",
                    )}
                  </p>
                )}
              </section>
            );
          })}

          {groups.roots.length === 0 && repositories.length > 0 ? (
            <section className="card border border-base-300 bg-base-100 shadow-sm">
              <header className="px-4 py-3 text-sm font-semibold">
                {t("manage.repositories.title", "Repositories")}
              </header>
              <ul className="list border-t border-base-200">{repositories.map(renderRow)}</ul>
            </section>
          ) : null}

          {groups.unlinked.length > 0 ? (
            <section className="card border border-warning/40 bg-base-100 shadow-sm">
              <header className="flex items-center gap-2 px-4 py-3">
                <TriangleAlert size={16} className="text-warning" />
                <div>
                  <div className="text-sm font-semibold">
                    {t("manage.repositories.unlinkedTitle", "Repositories needing attention")}
                  </div>
                  <p className="text-xs text-base-content/55">
                    {t(
                      "manage.repositories.unlinkedDescription",
                      "Their Storage Location is unknown to Lumilio. Reconnect or locate them from the row actions.",
                    )}
                  </p>
                </div>
              </header>
              <ul className="list border-t border-base-200">{groups.unlinked.map(renderRow)}</ul>
            </section>
          ) : null}
        </div>
      )}

      <AddRepositoryModal
        isOpen={isCreateOpen}
        onClose={() => setIsCreateOpen(false)}
        canRequestStorageLocation={nativeHostAvailable}
        showServerCandidates={!nativeHostAvailable}
        onRequestStorageLocation={() => {
          setIsCreateOpen(false);
          setHostAction({ kind: "authorize_storage_location" });
        }}
        onRecoveryRequired={(conflictType) => {
          if (conflictType === "repository_marker_invalid") {
            void navigate("/server-monitor?tab=storage");
          } else {
            setIsCandidateOpen(true);
          }
        }}
      />
      {hostAction ? (
        <NativeHostActionModal
          isOpen
          kind={hostAction.kind}
          rootId={hostAction.rootId}
          repositoryId={hostAction.repositoryId}
          onClose={() => setHostAction(null)}
        />
      ) : null}
      <RepositoryCandidateModal
        isOpen={isCandidateOpen}
        onClose={() => setIsCandidateOpen(false)}
      />
      <RemoveStorageLocationModal
        root={removeRootTarget}
        isOpen={removeRootTarget !== null}
        onClose={() => setRemoveRootTarget(null)}
      />
    </section>
  );
}

function isHostActionKind(value: string | undefined): value is HostActionKind {
  return (
    value === "authorize_storage_location" ||
    value === "open_repository" ||
    value === "locate_storage_location" ||
    value === "locate_repository"
  );
}

function rootStatusDot(status: string | undefined): string {
  switch (status) {
    case "active":
      return "status-success";
    case "maintenance":
      return "status-info";
    case "offline":
      return "status-warning";
    default:
      return "status-error";
  }
}

function storageRootStatusLabel(
  status: string | undefined,
  t: ReturnType<typeof useI18n>["t"],
): string {
  switch (status) {
    case "active":
      return t("manage.repositories.storageLocationAvailable", "Available");
    case "offline":
      return t("manage.repositories.storageLocationOffline", "Offline");
    case "maintenance":
      return t("manage.repositories.storageLocationMaintenanceBadge", "Maintenance");
    default:
      return t("manage.repositories.storageLocationErrorBadge", "Storage Location needs attention");
  }
}

function storageRootRemovalBlockedLabel(
  reason: string | undefined,
  t: ReturnType<typeof useI18n>["t"],
): string | undefined {
  switch (reason) {
    case "default_storage_location":
      return t(
        "manage.repositories.storageLocationRemoveBlockedDefault",
        "The default Storage Location cannot be removed.",
      );
    case "registered_repositories":
      return t(
        "manage.repositories.storageLocationRemoveBlockedRepositories",
        "Remove its repositories from Lumilio first.",
      );
    case "active_operation":
      return t(
        "manage.repositories.storageLocationRemoveBlockedOperation",
        "Wait for the active storage operation to finish.",
      );
    default:
      return undefined;
  }
}
