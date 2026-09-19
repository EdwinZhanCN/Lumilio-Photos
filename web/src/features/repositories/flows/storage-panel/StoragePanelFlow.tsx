import { useCallback, useMemo, useState } from "react";
import {
  CalendarRange,
  Download,
  FolderCheck,
  FolderOpen,
  FolderPlus,
  HardDrive,
  Link2Off,
  Users,
  Wrench,
} from "lucide-react";
import { CloudSourcesModal } from "@/features/cloud";
import { useMessage } from "@/features/notifications";
import AnchoredMenu, { MenuItem } from "@/components/ui/AnchoredMenu";
import PageHeader from "@/components/ui/PageHeader";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import {
  type HostActionKind,
  useNativeHostCapability,
  useUnfinishedNativeHostActions,
} from "../../api/useNativeHostActions";
import { useRepositoryDuplicateDetect } from "../../api/useRepositoryDuplicateDetect";
import { useRepositoryScan } from "../../api/useRepositoryScan";
import { useRepositoryVerificationCancel } from "../../api/useRepositoryVerifications";
import { useStorageSupportBundle } from "../../api/useStorageDiagnostics";
import { useStorageView } from "../../api/useStorageView";
import type { RepositoryRef, StorageLocationOption } from "../../types";
import AddRepositoryModal from "./AddRepositoryModal";
import NativeHostActionModal from "./NativeHostActionModal";
import RemoveRepositoryModal from "./RemoveRepositoryModal";
import RemoveStorageLocationModal from "./RemoveStorageLocationModal";
import RenameRepositoryModal from "./RenameRepositoryModal";
import RepositoryCandidateModal from "./RepositoryCandidateModal";
import RepositoryRowActions, { type RepositoryCommand } from "./RepositoryRowActions";
import RepositoryTable from "./RepositoryTable";
import StorageLocationSection from "./StorageLocationSection";
import StorageViewTabs, { type StorageViewMode } from "./StorageViewTabs";
import LifecycleHistory from "./LifecycleHistory";
import VerificationHistoryModal from "./VerificationHistoryModal";
import { formatClockTime } from "./storageTime";
import { buildStorageView, type RepositoryRow } from "./storageViewModel";

export type StorageMaintenanceCommands = {
  rebuildEvents: () => void;
  rebuildPeople: () => void;
  isRebuildingEvents: boolean;
  isRebuildingPeople: boolean;
};

/**
 * Administrator storage surface. One page, one structure: Storage Location
 * groups, Repository rows, and capacity as a Repository column carrying the
 * storage it was measured at. Capacity is deliberately absent from the page
 * header and from Location headers — a Location can hold Repositories on
 * different mounts, so a single figure there would misattribute free space.
 */
export default function StoragePanelFlow({
  maintenance,
}: {
  maintenance?: StorageMaintenanceCommands;
}) {
  const { t, i18n } = useI18n();
  const showMessage = useMessage();
  const viewQuery = useStorageView();
  const model = useMemo(() => buildStorageView(viewQuery.data), [viewQuery.data]);

  const { scanRepository, detectStacks, scanningIds } = useRepositoryScan();
  const { cancelVerification } = useRepositoryVerificationCancel();
  const { detectDuplicates } = useRepositoryDuplicateDetect();
  const locationRebuildMutation = $api.useMutation("post", "/api/v1/locations/rebuild");

  const supportBundle = useStorageSupportBundle();
  const nativeHostQuery = useNativeHostCapability();
  const nativeHostAvailable = nativeHostQuery.data?.available === true;
  const pendingHostActions = useUnfinishedNativeHostActions(nativeHostAvailable);

  const [mode, setMode] = useState<StorageViewMode>("browse");
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [createStorageLocationId, setCreateStorageLocationId] = useState<string | undefined>();
  const [isCandidateOpen, setIsCandidateOpen] = useState(false);
  const [removeLocation, setRemoveLocation] = useState<StorageLocationOption | null>(null);
  const [renameRepository, setRenameRepository] = useState<RepositoryRef | null>(null);
  const [removeRepository, setRemoveRepository] = useState<RepositoryRef | null>(null);
  const [cloudRepository, setCloudRepository] = useState<RepositoryRef | null>(null);
  const [hostAction, setHostAction] = useState<{
    kind: HostActionKind;
    storageLocationId?: string;
    repositoryId?: string;
  } | null>(null);
  const [verificationHistoryRepository, setVerificationHistoryRepository] = useState<{
    id: string;
    name: string;
  } | null>(null);

  const runCommand = useCallback(
    async (command: RepositoryCommand, row: RepositoryRow) => {
      const ref: RepositoryRef = { id: row.id, rawName: row.name, role: row.role };
      switch (command) {
        case "verify":
          try {
            const result = await scanRepository(row.id);
            showMessage(
              result.coalesced ? "info" : "success",
              result.coalesced
                ? t(
                    "manage.repositories.scanCoalesced",
                    "A scan is already active for {{name}}; this request joined the existing operation.",
                    { name: row.name },
                  )
                : t(
                    "manage.repositories.scanQueued",
                    "Scan queued for {{name}}. You can keep using Lumilio while it runs.",
                    { name: row.name },
                  ),
            );
          } catch (error) {
            showMessage("error", localizeAPIProblem(error, t, t("manage.repositories.scanFailed")));
          }
          return;
        case "cancelVerification":
          try {
            await cancelVerification(row.id);
            showMessage(
              "success",
              t("storagePanel.cancelVerificationSuccess", "Cancellation requested for {{name}}.", {
                name: row.name,
              }),
            );
          } catch (error) {
            showMessage(
              "error",
              localizeAPIProblem(
                error,
                t,
                t("storagePanel.cancelVerificationFailed", "The scan could not be cancelled."),
              ),
            );
          }
          return;
        case "verificationHistory":
          setVerificationHistoryRepository({ id: row.id, name: row.name });
          return;
        case "detectStacks":
          try {
            const created = await detectStacks(row.id);
            showMessage(
              "success",
              t("manage.repositories.detectStacksCompleted", {
                name: row.name,
                count: created,
              }),
            );
          } catch (error) {
            showMessage(
              "error",
              localizeAPIProblem(error, t, t("manage.repositories.detectStacksFailed")),
            );
          }
          return;
        case "detectDuplicates":
          try {
            const result = await detectDuplicates(row.id);
            showMessage(
              "success",
              t(
                "storagePanel.detectDuplicatesCompleted",
                "Duplicate scan finished for {{name}}: {{groups}} groups found.",
                { name: row.name, groups: result?.groups ?? 0 },
              ),
            );
          } catch (error) {
            showMessage(
              "error",
              localizeAPIProblem(
                error,
                t,
                t(
                  "storagePanel.detectDuplicatesFailed",
                  "Duplicate detection could not be completed.",
                ),
              ),
            );
          }
          return;
        case "rebuildLocations":
          try {
            await locationRebuildMutation.mutateAsync({ body: { repository_id: row.id } });
            showMessage(
              "success",
              t(
                "manage.repositories.locationRebuildQueued",
                "Location rebuild queued for {{name}}.",
                { name: row.name },
              ),
            );
          } catch (error) {
            showMessage(
              "error",
              localizeAPIProblem(
                error,
                t,
                t(
                  "manage.repositories.locationRebuildFailed",
                  "Location rebuild could not be queued.",
                ),
              ),
            );
          }
          return;
        case "rename":
          setRenameRepository(ref);
          return;
        case "remove":
          setRemoveRepository(ref);
          return;
        case "cloudSources":
          setCloudRepository(ref);
          return;
        case "locate":
          setHostAction({ kind: "locate_repository", repositoryId: row.id });
          return;
      }
    },
    [
      cancelVerification,
      detectDuplicates,
      detectStacks,
      locationRebuildMutation,
      scanRepository,
      showMessage,
      t,
    ],
  );

  const renderRowActions = useCallback(
    (row: RepositoryRow) => (
      <RepositoryRowActions
        row={row}
        onCommand={(command, target) => void runCommand(command, target)}
        nativeHostAvailable={nativeHostAvailable}
        isScanning={scanningIds.has(row.id)}
      />
    ),
    [nativeHostAvailable, runCommand, scanningIds],
  );

  const openAuthorizeLocation = () => {
    if (nativeHostAvailable) {
      setHostAction({ kind: "authorize_storage_location" });
    } else {
      showMessage(
        "info",
        t(
          "storagePanel.authorizeNativeOnly",
          "Authorizing a Storage Location requires the Lumilio desktop app. On a Server deployment, mount the directory inside the configured storage path instead.",
        ),
      );
    }
  };

  const observedAt = model.observedAt ? formatClockTime(model.observedAt, i18n.language) : null;
  const pendingCount = pendingHostActions.data?.length ?? 0;
  const summary = [
    t("storagePanel.summaryLocations", "{{count}} Storage Locations", {
      count: model.locations.length,
    }),
    t("storagePanel.summaryRepositories", "{{count}} Repositories", {
      count: model.repositoryCount,
    }),
    model.attentionCount > 0
      ? t("storagePanel.summaryAttention", "{{count}} need attention", {
          count: model.attentionCount,
        })
      : null,
    observedAt
      ? t("storagePanel.summaryObserved", "measured at {{time}}", { time: observedAt })
      : null,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader
        title={t("storagePanel.title", "Storage")}
        subtitle={summary}
        icon={<HardDrive className="size-5 text-primary" />}
      >
        <button
          type="button"
          className="btn btn-primary btn-sm"
          onClick={() => setIsCreateOpen(true)}
        >
          <FolderPlus className="size-4" aria-hidden />
          {t("storagePanel.action.addRepository", "Add Repository")}
        </button>
        <AnchoredMenu
          label={t("storagePanel.moreActions", "More storage actions")}
          menuWidth={256}
          menuHeight={maintenance ? 260 : 190}
          triggerClassName="btn btn-square btn-ghost btn-sm"
        >
          {({ close }) => (
            <>
              <MenuItem
                onSelect={() => {
                  close();
                  setIsCandidateOpen(true);
                }}
              >
                <FolderOpen className="size-4" aria-hidden />
                {t("manage.repositories.openExisting", "Open Existing Repository")}
              </MenuItem>
              <MenuItem
                onSelect={() => {
                  close();
                  openAuthorizeLocation();
                }}
              >
                <FolderCheck className="size-4" aria-hidden />
                {t("storagePanel.authorize", "Authorize Storage Location")}
              </MenuItem>
              <MenuItem
                disabled={supportBundle.isFetching}
                onSelect={() => {
                  close();
                  void supportBundle.refetch();
                }}
              >
                <Download className="size-4" aria-hidden />
                {t("storagePanel.action.supportBundle", "Download support bundle")}
              </MenuItem>
              {maintenance ? (
                <>
                  <li role="none" className="menu-title">
                    {t("storagePanel.maintenance", "Maintenance")}
                  </li>
                  <MenuItem
                    disabled={maintenance.isRebuildingEvents}
                    onSelect={() => {
                      close();
                      maintenance.rebuildEvents();
                    }}
                  >
                    <CalendarRange className="size-4" aria-hidden />
                    {t("manage.repositories.rebuildEvents", "Rebuild events")}
                  </MenuItem>
                  <MenuItem
                    disabled={maintenance.isRebuildingPeople}
                    onSelect={() => {
                      close();
                      maintenance.rebuildPeople();
                    }}
                  >
                    <Users className="size-4" aria-hidden />
                    {t("manage.repositories.rebuildPeople", "Rebuild people")}
                  </MenuItem>
                </>
              ) : null}
            </>
          )}
        </AnchoredMenu>
      </PageHeader>

      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-3 pb-6 pt-3 sm:px-4">
        <StorageViewTabs mode={mode} onModeChange={setMode} />

        {viewQuery.isError ? (
          <div role="alert" className="alert alert-error alert-soft">
            <span>{t("storagePanel.loadFailed", "Storage view could not be loaded.")}</span>
            <button type="button" className="btn btn-sm" onClick={() => void viewQuery.refetch()}>
              {t("common.retry", "Retry")}
            </button>
          </div>
        ) : null}

        {pendingCount > 0 ? (
          <div role="alert" className="alert alert-info alert-soft">
            <Wrench className="size-4" aria-hidden />
            <span>
              {t(
                "storagePanel.pendingHostActions",
                "{{count}} storage request(s) are waiting for approval in the Desktop app.",
                { count: pendingCount },
              )}
            </span>
          </div>
        ) : null}

        {viewQuery.isLoading ? (
          <div className="flex flex-col gap-2">
            {[0, 1, 2].map((index) => (
              <div key={index} className="skeleton h-16 w-full" />
            ))}
          </div>
        ) : mode === "history" ? (
          <section className="overflow-hidden rounded-box border border-base-300 bg-base-100">
            <LifecycleHistory />
          </section>
        ) : (
          <>
            {model.locations.map((group) => (
              <StorageLocationSection
                key={group.id}
                group={group}
                expandedId={expandedId}
                onToggleRow={(id) => setExpandedId((current) => (current === id ? null : id))}
                renderActions={renderRowActions}
                onAddRepository={() => {
                  setCreateStorageLocationId(group.id);
                  setIsCreateOpen(true);
                }}
                onRemoveLocation={() =>
                  setRemoveLocation({
                    id: group.id,
                    entityType: "storage_location",
                    kind: group.kind,
                    rawName: group.name,
                    path: "",
                    can_remove: group.canRemove,
                  } as StorageLocationOption)
                }
              />
            ))}

            {model.unlinked.length > 0 ? (
              <section className="overflow-hidden rounded-box border border-warning/40 bg-base-100">
                <header className="flex items-start gap-2 px-4 py-3">
                  <Link2Off
                    className="mt-0.5 size-4 shrink-0 text-warning"
                    strokeWidth={1.6}
                    aria-hidden
                  />
                  <div className="min-w-0">
                    <h2 className="text-sm font-semibold">
                      {t("storagePanel.unlinked.title", "Repositories without a Storage Location")}
                    </h2>
                    <p className="mt-0.5 text-xs text-base-content/55">
                      {t(
                        "storagePanel.unlinked.description",
                        "Their Storage Location is not registered here. Files are untouched; reconnecting or locating them restores access.",
                      )}
                    </p>
                  </div>
                </header>
                <div className="border-t border-base-200">
                  <RepositoryTable
                    rows={model.unlinked}
                    expandedId={expandedId}
                    onToggleRow={(id) => setExpandedId((current) => (current === id ? null : id))}
                    renderActions={renderRowActions}
                  />
                </div>
              </section>
            ) : null}
          </>
        )}
      </div>

      <AddRepositoryModal
        isOpen={isCreateOpen}
        onClose={() => {
          setIsCreateOpen(false);
          setCreateStorageLocationId(undefined);
        }}
        canRequestStorageLocation={nativeHostAvailable}
        onRequestStorageLocation={() => setHostAction({ kind: "authorize_storage_location" })}
        showServerCandidates={!nativeHostAvailable}
        initialStorageLocationId={createStorageLocationId}
      />
      <RepositoryCandidateModal
        isOpen={isCandidateOpen}
        onClose={() => setIsCandidateOpen(false)}
      />
      <NativeHostActionModal
        isOpen={hostAction != null}
        kind={hostAction?.kind ?? "open_repository"}
        storageLocationId={hostAction?.storageLocationId}
        repositoryId={hostAction?.repositoryId}
        onClose={() => setHostAction(null)}
      />
      <RemoveStorageLocationModal
        root={removeLocation}
        isOpen={removeLocation != null}
        onClose={() => setRemoveLocation(null)}
      />
      {renameRepository ? (
        <RenameRepositoryModal
          repository={renameRepository}
          isOpen
          onClose={() => setRenameRepository(null)}
        />
      ) : null}
      {removeRepository ? (
        <RemoveRepositoryModal
          repository={removeRepository}
          isOpen
          onClose={() => setRemoveRepository(null)}
        />
      ) : null}
      {cloudRepository ? (
        <CloudSourcesModal
          repositoryId={cloudRepository.id}
          repositoryName={cloudRepository.rawName}
          isOpen
          onClose={() => setCloudRepository(null)}
        />
      ) : null}
      {verificationHistoryRepository ? (
        <VerificationHistoryModal
          repositoryId={verificationHistoryRepository.id}
          repositoryName={verificationHistoryRepository.name}
          isOpen
          onClose={() => setVerificationHistoryRepository(null)}
        />
      ) : null}
    </div>
  );
}
