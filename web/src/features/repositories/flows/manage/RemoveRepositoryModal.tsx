import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { ArchiveX, CloudDownload, Database, FolderHeart, ListTodo, X } from "lucide-react";
import { useMessage } from "@/features/notifications";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import { useRemoveRepository } from "../../api/useRemoveRepository";
import type { RepositoryOption } from "../../types";

export default function RemoveRepositoryModal({
  repository,
  isOpen,
  onClose,
}: {
  repository: RepositoryOption;
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [confirmationName, setConfirmationName] = useState("");
  const removal = useRemoveRepository();
  const impactQuery = $api.useQuery(
    "get",
    "/api/v1/repositories/{id}/removal-impact",
    { params: { path: { id: repository.id } } },
    { enabled: isOpen && Boolean(repository.id), staleTime: 0 },
  );

  useEffect(() => {
    if (!isOpen) setConfirmationName("");
  }, [isOpen]);

  if (!isOpen) return null;

  const impact = impactQuery.data;
  const confirmed = confirmationName === repository.rawName;
  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!confirmed || removal.isPending) return;
    try {
      await removal.removeRepository(repository.id, confirmationName);
      showMessage(
        "success",
        t(
          "manage.repositories.removeSuccess",
          'Removed "{{name}}" from Lumilio. Files on disk were preserved.',
          { name: repository.rawName },
        ),
      );
      setConfirmationName("");
      onClose();
    } catch (error) {
      showMessage(
        "error",
        localizeAPIProblem(
          error,
          t,
          t("manage.repositories.removeFailed", "Could not remove this Repository."),
        ),
      );
    }
  };

  return (
    <div className="modal modal-open z-modal">
      <div className="modal-box max-w-lg">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg bg-warning/15 text-warning">
              <ArchiveX size={20} />
            </div>
            <div>
              <h3 className="text-base font-semibold">
                {t("manage.repositories.removeTitle", "Remove from Lumilio")}
              </h3>
              <p className="text-sm text-base-content/60">{repository.rawName}</p>
            </div>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle"
            onClick={onClose}
            disabled={removal.isPending}
            aria-label={t("common.close", "Close")}
          >
            <X size={18} />
          </button>
        </div>

        <div className="mt-5 rounded-lg border border-success/25 bg-success/10 p-4 text-sm">
          <p className="font-medium text-success">
            {t(
              "manage.repositories.removeSafetyWarning",
              "Files on disk will be preserved; some metadata in the Lumilio catalog may not be recoverable after reopening this Repository.",
            )}
          </p>
        </div>

        {impactQuery.isLoading ? (
          <div className="flex h-32 items-center justify-center">
            <span className="loading loading-spinner loading-md" />
          </div>
        ) : impactQuery.isError ? (
          <div className="mt-4 rounded-lg border border-error/30 bg-error/10 p-3 text-sm text-error">
            {t("manage.repositories.removeImpactFailed", "Could not load removal impact.")}
          </div>
        ) : (
          <div className="mt-4 grid grid-cols-2 gap-2 text-sm">
            <ImpactItem
              icon={<Database size={16} />}
              label={t("manage.repositories.removeAssetCount", "Catalog assets")}
              value={(impact?.asset_count ?? 0).toLocaleString()}
            />
            <ImpactItem
              icon={<FolderHeart size={16} />}
              label={t("manage.repositories.removeAlbumCount", "Affected albums")}
              value={(impact?.album_count ?? 0).toLocaleString()}
            />
            <ImpactItem
              icon={<ListTodo size={16} />}
              label={t("manage.repositories.removeTaskCount", "Queued or active tasks")}
              value={(impact?.active_task_count ?? 0).toLocaleString()}
            />
            <ImpactItem
              icon={<CloudDownload size={16} />}
              label={t("manage.repositories.removeCloudImportCount", "Cloud import receipts")}
              value={(impact?.cloud_import_count ?? 0).toLocaleString()}
            />
            <ImpactItem
              icon={<ArchiveX size={16} />}
              label={t("manage.repositories.removeCatalogBytes", "Cataloged media size")}
              value={formatBytes(impact?.catalog_media_bytes ?? 0)}
            />
          </div>
        )}

        <form className="mt-5" onSubmit={handleSubmit}>
          <label className="fieldset w-full gap-1" htmlFor="remove-repository-confirmation">
            <span className="fieldset-legend p-0 text-sm font-medium">
              {t("manage.repositories.removeConfirmationLabel", 'Type "{{name}}" to confirm', {
                name: repository.rawName,
              })}
            </span>
            <input
              id="remove-repository-confirmation"
              className="input input-bordered w-full"
              value={confirmationName}
              onChange={(event) => setConfirmationName(event.target.value)}
              autoComplete="off"
            />
          </label>
          <div className="modal-action">
            <button
              type="button"
              className="btn btn-ghost"
              onClick={onClose}
              disabled={removal.isPending}
            >
              {t("common.cancel", "Cancel")}
            </button>
            <button
              type="submit"
              className="btn btn-warning"
              disabled={
                !confirmed || impactQuery.isLoading || impactQuery.isError || removal.isPending
              }
            >
              {removal.isPending && <span className="loading loading-spinner loading-xs" />}
              {t("manage.repositories.removeAction", "Remove from Lumilio")}
            </button>
          </div>
        </form>
      </div>
      <button
        type="button"
        className="modal-backdrop"
        onClick={onClose}
        aria-label={t("common.close", "Close")}
      />
    </div>
  );
}

function ImpactItem({ icon, label, value }: { icon: ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-lg border border-base-300 p-3">
      <div className="flex items-center gap-2 text-base-content/55">
        {icon}
        <span>{label}</span>
      </div>
      <div className="mt-1 font-semibold tabular-nums">{value}</div>
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** exponent;
  return `${value.toLocaleString(undefined, { maximumFractionDigits: 1 })} ${units[exponent]}`;
}
