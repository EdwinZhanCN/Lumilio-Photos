import { HardDrive, X } from "lucide-react";
import { useMessage } from "@/features/notifications";
import { formatBytes } from "@/lib/utils/formatters";
import type { components } from "@/lib/http-commons/schema";
import { useI18n } from "@/lib/i18n";
import { useRemoveStorageLocation } from "../../api/useRemoveStorageLocation";

type RepositoryRoot = components["schemas"]["dto.RepositoryRootDTO"];

export default function RemoveStorageLocationModal({
  root,
  isOpen,
  onClose,
}: {
  root: RepositoryRoot | null;
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const removeStorageLocation = useRemoveStorageLocation();

  if (!isOpen || !root || !root.id) return null;

  const handleRemove = async () => {
    if (!root.id || !root.can_remove || removeStorageLocation.isPending) return;
    try {
      await removeStorageLocation.mutateAsync({ params: { path: { id: root.id } } });
      showMessage(
        "success",
        t("manage.repositories.storageLocationRemoved", "Storage Location removed from Lumilio."),
      );
      onClose();
    } catch (reason: unknown) {
      showMessage(
        "error",
        reason instanceof Error
          ? reason.message
          : t(
              "manage.repositories.storageLocationRemoveFailed",
              "Storage Location could not be removed.",
            ),
      );
    }
  };

  return (
    <div className="modal modal-open z-modal">
      <div className="modal-box max-w-md">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg bg-warning/15 text-warning">
              <HardDrive size={20} />
            </div>
            <div>
              <h3 className="text-base font-semibold">
                {t("manage.repositories.storageLocationRemoveTitle", "Remove Storage Location")}
              </h3>
              <p className="text-sm text-base-content/60">{root.name}</p>
            </div>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle"
            onClick={onClose}
            disabled={removeStorageLocation.isPending}
            aria-label={t("common.close", { defaultValue: "Close" })}
          >
            <X size={18} />
          </button>
        </div>

        <p className="mt-4 text-sm text-base-content/70">
          {t(
            "manage.repositories.storageLocationRemoveConfirmation",
            'Remove "{{name}}" from Lumilio? Its folder, .lumilioroot marker, and every file on disk will be preserved.',
            { name: root.name ?? "" },
          )}
        </p>

        <div className="mt-4 space-y-1 rounded-lg border border-base-300 bg-base-200/40 px-3 py-2 text-xs text-base-content/65">
          <div className="break-all font-mono">{root.path}</div>
          <div>
            {root.capacity_known
              ? t(
                  "manage.repositories.storageLocationCapacity",
                  "{{available}} available of {{total}}",
                  {
                    available: formatBytes(root.available_bytes ?? 0),
                    total: formatBytes(root.total_bytes ?? 0),
                  },
                )
              : t("manage.repositories.storageLocationCapacityUnknown", "Capacity unavailable")}
            {` · ${t(
              "manage.repositories.storageLocationRepositoryCount",
              "{{count}} repositories",
              { count: root.repository_count ?? 0 },
            )}`}
          </div>
        </div>

        <div className="modal-action">
          <button
            type="button"
            className="btn btn-ghost"
            onClick={onClose}
            disabled={removeStorageLocation.isPending}
          >
            {t("common.cancel", { defaultValue: "Cancel" })}
          </button>
          <button
            type="button"
            className="btn btn-warning"
            disabled={!root.can_remove || removeStorageLocation.isPending}
            onClick={() => void handleRemove()}
          >
            {removeStorageLocation.isPending && (
              <span className="loading loading-spinner loading-xs" />
            )}
            {t("manage.repositories.storageLocationRemove", "Remove")}
          </button>
        </div>
      </div>
      <button
        type="button"
        className="modal-backdrop"
        onClick={onClose}
        aria-label={t("common.close", { defaultValue: "Close" })}
      />
    </div>
  );
}
