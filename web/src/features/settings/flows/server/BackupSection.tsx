import { useEffect, useRef, useState } from "react";
import {
  ArchiveRestoreIcon,
  DatabaseBackupIcon,
  DownloadIcon,
  RefreshCwIcon,
  Trash2Icon,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { formatBytes } from "@/lib/utils/formatters";
import { useSystemSettings, useUpdateSystemSettings } from "../../api/useSystemSettings";
import {
  downloadBackup,
  useBackups,
  useCreateBackup,
  useDeleteBackup,
  useRestoreBackup,
  useLatestRestoreOperation,
  useRestoreOperation,
  type BackupEntry,
} from "../../api/useBackups";
import { SettingsGroup, SettingsRow, SettingsBlock } from "../../components/SettingsGroup";
import {
  clearRestoreOperationID,
  readRestoreOperationID,
  saveRestoreOperationID,
} from "../../state/restoreOperation";

const intervalPresets = [6, 12, 24, 48, 168];

/** Settings → Server: automatic database-backup schedule/retention plus the
 * dump list (create now / download / restore / delete). Restore is observed as
 * a durable operation because the Server restarts between file-swap phases. */
export default function BackupSection() {
  const { t } = useI18n();
  const settingsQuery = useSystemSettings();
  const updateSettings = useUpdateSystemSettings();

  // Poll the list briefly after "back up now": the dump appears when the
  // background job finishes.
  const [pollUntil, setPollUntil] = useState(0);
  const backupsQuery = useBackups(Date.now() < pollUntil);
  const createBackup = useCreateBackup();
  const deleteBackup = useDeleteBackup();
  const restoreBackup = useRestoreBackup();
  const [restoreOperationID, setRestoreOperationID] = useState<string | null>(
    readRestoreOperationID,
  );
  const latestRestoreQuery = useLatestRestoreOperation(!restoreOperationID);
  const restoreOperationQuery = useRestoreOperation(restoreOperationID);

  // Name of the entry whose destructive action awaits its second click.
  const [confirming, setConfirming] = useState<{
    name: string;
    action: "delete" | "restore";
  } | null>(null);
  const [error, setError] = useState<string | null>(null);
  const downloadBusy = useRef(false);
  const reloadScheduled = useRef(false);
  const restoring = Boolean(restoreOperationID) || restoreBackup.isPending;

  const backup = settingsQuery.data?.backup;
  const backups = backupsQuery.data?.backups ?? [];
  const restoreOperation = restoreOperationQuery.data;

  useEffect(() => {
    const latest = latestRestoreQuery.data;
    if (
      restoreOperationID ||
      !latest ||
      latest.status === "completed" ||
      latest.status === "rolled_back" ||
      latest.status === "failed"
    ) {
      return;
    }
    saveRestoreOperationID(latest.id);
    setRestoreOperationID(latest.id);
  }, [latestRestoreQuery.data, restoreOperationID]);

  useEffect(() => {
    if (!restoreOperationID || !restoreOperation) return;
    switch (restoreOperation.status) {
      case "completed":
        if (!reloadScheduled.current) {
          reloadScheduled.current = true;
          clearRestoreOperationID();
          window.setTimeout(() => window.location.reload(), 750);
        }
        break;
      case "rolled_back":
      case "failed":
        clearRestoreOperationID();
        setRestoreOperationID(null);
        setError(
          t("settings.serverSettings.backup.restoreFailed", {
            defaultValue: "Restore failed. The previous database remains active.",
          }),
        );
        break;
      default:
        break;
    }
  }, [restoreOperation, restoreOperationID, t]);

  const patchBackup = (patch: {
    enabled?: boolean;
    interval_hours?: number;
    keep_last?: number;
  }) => {
    setError(null);
    updateSettings.mutate({ body: { backup: patch } });
  };

  const onCreate = () => {
    setError(null);
    createBackup.mutate({}, { onSuccess: () => setPollUntil(Date.now() + 30_000) });
  };

  const onDownload = async (name: string) => {
    if (downloadBusy.current) return;
    downloadBusy.current = true;
    setError(null);
    try {
      await downloadBackup(name);
    } catch {
      setError(
        t("settings.serverSettings.backup.downloadFailed", { defaultValue: "Download failed." }),
      );
    } finally {
      downloadBusy.current = false;
    }
  };

  const onConfirmedAction = (entry: BackupEntry) => {
    const name = entry.name ?? "";
    if (confirming?.action === "delete") {
      setConfirming(null);
      deleteBackup.mutate(
        { params: { path: { name } } },
        {
          onError: () =>
            setError(
              t("settings.serverSettings.backup.deleteFailed", { defaultValue: "Delete failed." }),
            ),
        },
      );
      return;
    }
    setConfirming(null);
    setError(null);
    restoreBackup.mutate(name, {
      onSuccess: (operation) => {
        saveRestoreOperationID(operation.id);
        setRestoreOperationID(operation.id);
      },
      onError: (cause) => {
        setError(
          cause instanceof Error
            ? cause.message
            : t("settings.serverSettings.backup.restoreFailed", {
                defaultValue: "Restore could not be started.",
              }),
        );
      },
    });
  };

  return (
    <SettingsGroup
      title={t("settings.serverSettings.backup.title", { defaultValue: "Database backups" })}
      description={t("settings.serverSettings.backup.description", {
        defaultValue:
          "Automatic snapshots of the metadata database (albums, people, edits) stored in the configured backups folder. Repository media files are not included.",
      })}
    >
      <SettingsRow
        icon={<DatabaseBackupIcon className="size-4" />}
        iconColor="bg-info text-info-content"
        label={t("settings.serverSettings.backup.enabledLabel", {
          defaultValue: "Automatic backups",
        })}
        control={
          <input
            type="checkbox"
            className="toggle toggle-primary"
            checked={backup?.enabled ?? true}
            disabled={!backup || updateSettings.isPending}
            aria-label={t("settings.serverSettings.backup.enabledLabel", {
              defaultValue: "Automatic backups",
            })}
            onChange={(event) => patchBackup({ enabled: event.target.checked })}
          />
        }
      />
      <SettingsRow
        label={t("settings.serverSettings.backup.intervalLabel", { defaultValue: "Backup every" })}
        control={
          <select
            className="select select-sm select-bordered"
            value={backup?.interval_hours ?? 24}
            disabled={!backup || updateSettings.isPending}
            onChange={(event) => patchBackup({ interval_hours: Number(event.target.value) })}
          >
            {intervalPresets.map((hours) => (
              <option key={hours} value={hours}>
                {t("settings.serverSettings.backup.intervalHours", {
                  defaultValue: "{{hours}} h",
                  hours,
                })}
              </option>
            ))}
          </select>
        }
      />
      <SettingsRow
        label={t("settings.serverSettings.backup.keepLastLabel", { defaultValue: "Keep last" })}
        control={
          <input
            type="number"
            className="input input-sm input-bordered w-20 text-right"
            min={1}
            max={365}
            value={backup?.keep_last ?? 14}
            disabled={!backup || updateSettings.isPending}
            onChange={(event) => {
              const parsed = Number(event.target.value);
              if (Number.isFinite(parsed) && parsed >= 1) {
                patchBackup({ keep_last: Math.min(365, Math.trunc(parsed)) });
              }
            }}
          />
        }
      />

      <SettingsBlock>
        <div className="mb-3 flex items-center justify-between">
          <span className="text-sm font-medium">
            {t("settings.serverSettings.backup.listTitle", { defaultValue: "Available backups" })}
          </span>
          <div className="flex items-center gap-2">
            <button
              className="btn btn-ghost btn-xs"
              onClick={() => backupsQuery.refetch()}
              aria-label={t("settings.serverSettings.backup.refresh", { defaultValue: "Refresh" })}
            >
              <RefreshCwIcon className="size-3.5" />
            </button>
            <button
              className="btn btn-primary btn-xs"
              disabled={createBackup.isPending}
              onClick={onCreate}
            >
              {t("settings.serverSettings.backup.createNow", { defaultValue: "Back up now" })}
            </button>
          </div>
        </div>

        {error && <p className="mb-2 text-sm text-error">{error}</p>}
        {restoring && (
          <div className="mb-2 rounded-box border border-warning/30 bg-warning/10 p-3 text-sm">
            <p className="font-medium text-warning">
              {t("settings.serverSettings.backup.restoring", {
                defaultValue: "Database restore in progress",
              })}
            </p>
            <p className="mt-1 text-base-content/70">
              {restoreOperation
                ? t(`settings.serverSettings.backup.restorePhases.${restoreOperation.status}`, {
                    defaultValue: restoreOperation.message,
                  })
                : t("settings.serverSettings.backup.restoreDisconnect", {
                    defaultValue:
                      "Lumilio Photos may be temporarily unreachable while it restarts. Keep this page open; progress resumes automatically.",
                  })}
            </p>
            {restoreOperation?.status && (
              <p className="mt-1 font-mono text-xs text-base-content/50">
                {restoreOperation.status}
              </p>
            )}
          </div>
        )}
        {createBackup.isSuccess && Date.now() < pollUntil && (
          <p className="mb-2 text-sm text-base-content/60">
            {t("settings.serverSettings.backup.queued", {
              defaultValue: "Backup queued — it appears below when finished.",
            })}
          </p>
        )}

        {backupsQuery.isLoading ? (
          <p className="text-sm text-base-content/60">{t("common.loading")}</p>
        ) : backupsQuery.isError ? (
          <div className="rounded-box border border-error/30 bg-error/10 p-3 text-sm">
            <p className="text-error">
              {t("settings.serverSettings.backup.listFailed", {
                defaultValue: "Backups could not be loaded.",
              })}
            </p>
            <button className="btn btn-ghost btn-xs mt-2" onClick={() => backupsQuery.refetch()}>
              {t("common.retry", { defaultValue: "Retry" })}
            </button>
          </div>
        ) : backups.length === 0 ? (
          <p className="text-sm text-base-content/60">
            {t("settings.serverSettings.backup.empty", { defaultValue: "No backups yet." })}
          </p>
        ) : (
          <ul className="divide-y divide-base-200">
            {backups.map((entry) => {
              const name = entry.name ?? "";
              const isConfirming = confirming?.name === name;
              return (
                <li key={name} className="flex items-center gap-3 py-2">
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-mono text-xs">{name}</p>
                    <p className="text-xs text-base-content/55">
                      {entry.created_at ? new Date(entry.created_at).toLocaleString() : "—"}
                      {" · "}
                      {formatBytes(entry.size_bytes ?? 0)}
                      {" · v"}
                      {entry.app_version}
                      {" · SQLite "}
                      {entry.sqlite_version}
                      {entry.restore_point && (
                        <span className="badge badge-ghost badge-xs ml-2 align-middle">
                          {t("settings.serverSettings.backup.restorePoint", {
                            defaultValue: "restore point",
                          })}
                        </span>
                      )}
                    </p>
                  </div>
                  {isConfirming ? (
                    <div className="flex shrink-0 items-center gap-1">
                      <span className="text-xs text-warning">
                        {confirming.action === "restore"
                          ? t("settings.serverSettings.backup.confirmRestore", {
                              defaultValue: "Replace the current database?",
                            })
                          : t("settings.serverSettings.backup.confirmDelete", {
                              defaultValue: "Delete this backup?",
                            })}
                      </span>
                      <button
                        className="btn btn-error btn-xs"
                        onClick={() => onConfirmedAction(entry)}
                      >
                        {t("settings.serverSettings.backup.confirmYes", {
                          defaultValue: "Confirm",
                        })}
                      </button>
                      <button className="btn btn-ghost btn-xs" onClick={() => setConfirming(null)}>
                        {t("settings.serverSettings.backup.confirmNo", { defaultValue: "Cancel" })}
                      </button>
                    </div>
                  ) : (
                    <div className="flex shrink-0 items-center gap-1">
                      <button
                        className="btn btn-ghost btn-xs"
                        onClick={() => onDownload(name)}
                        aria-label={t("settings.serverSettings.backup.download", {
                          defaultValue: "Download",
                        })}
                      >
                        <DownloadIcon className="size-3.5" />
                      </button>
                      <button
                        className="btn btn-ghost btn-xs"
                        disabled={restoring}
                        onClick={() => setConfirming({ name, action: "restore" })}
                        aria-label={t("settings.serverSettings.backup.restore", {
                          defaultValue: "Restore",
                        })}
                      >
                        <ArchiveRestoreIcon className="size-3.5" />
                      </button>
                      <button
                        className="btn btn-ghost btn-xs text-error"
                        disabled={deleteBackup.isPending}
                        onClick={() => setConfirming({ name, action: "delete" })}
                        aria-label={t("settings.serverSettings.backup.delete", {
                          defaultValue: "Delete",
                        })}
                      >
                        <Trash2Icon className="size-3.5" />
                      </button>
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        )}
      </SettingsBlock>
    </SettingsGroup>
  );
}
