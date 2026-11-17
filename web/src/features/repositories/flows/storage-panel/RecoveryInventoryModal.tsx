import { useState } from "react";
import { Link } from "react-router-dom";
import { Archive, Download, Trash2 } from "lucide-react";
import Modal from "@/components/ui/Modal";
import { useMessage } from "@/features/notifications";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import { useI18n } from "@/lib/i18n";
import { formatBytes } from "@/lib/utils/formatters";
import {
  exportRecoveryItem,
  recoveryItemFilename,
  useDeleteRecoveryItem,
  useRepositoryRecovery,
  type RepositoryRecoveryItem,
} from "../../api/useRepositoryRecovery";
import { recoveryClassLabel, recoveryDispositionLabel } from "./recoveryCopy";
import { formatDateTime } from "./storageTime";

type Props = {
  repositoryId: string;
  repositoryName: string;
  isOpen: boolean;
  onClose: () => void;
};

/**
 * The private state a Repository carried into its registration, by class:
 * held Trash (decided in the Trash view), Studio records attached or awaiting
 * review, unfinished uploads and unknown files kept for export. Every change
 * here is an explicit decision on one item: export leaves it in place, delete
 * removes it and is recorded in the receipt. Nothing expires on its own.
 */
export default function RecoveryInventoryModal({
  repositoryId,
  repositoryName,
  isOpen,
  onClose,
}: Props) {
  const { t, i18n } = useI18n();
  const showMessage = useMessage();
  const inventory = useRepositoryRecovery(repositoryId, isOpen);
  const deleteItem = useDeleteRecoveryItem(repositoryId);
  const [pendingDelete, setPendingDelete] = useState<RepositoryRecoveryItem | null>(null);
  const [exporting, setExporting] = useState<string | null>(null);

  const items = inventory.data?.items ?? [];
  const receipts = inventory.data?.receipts ?? [];
  const held = inventory.data?.trash?.held_count ?? 0;
  const failedCopy = t(
    "storagePanel.recovery.actionFailed",
    "The recovery action could not be completed.",
  );

  const runExport = async (item: RepositoryRecoveryItem) => {
    if (!item.id) return;
    setExporting(item.id);
    try {
      await exportRecoveryItem(repositoryId, item);
    } catch (error) {
      showMessage("error", localizeAPIProblem(error, t, failedCopy));
    } finally {
      setExporting(null);
    }
  };

  const confirmDelete = async () => {
    if (!pendingDelete?.id || deleteItem.isPending) return;
    try {
      await deleteItem.mutateAsync({
        params: { path: { id: repositoryId, item: pendingDelete.id } },
      });
      showMessage(
        "success",
        t("storagePanel.recovery.deleted", "{{name}} was deleted.", {
          name: recoveryItemFilename(pendingDelete),
        }),
      );
      setPendingDelete(null);
    } catch (error) {
      showMessage("error", localizeAPIProblem(error, t, failedCopy));
    }
  };

  return (
    <Modal
      open={isOpen}
      onClose={onClose}
      dismissable={!deleteItem.isPending}
      size="lg"
      icon={<Archive className="size-5" aria-hidden />}
      title={t("storagePanel.recovery.title", "Recovery inventory")}
      footer={
        pendingDelete ? (
          <>
            <button
              type="button"
              className="btn btn-ghost"
              disabled={deleteItem.isPending}
              onClick={() => setPendingDelete(null)}
            >
              {t("common.cancel", "Cancel")}
            </button>
            <button
              type="button"
              className="btn btn-error"
              disabled={deleteItem.isPending}
              onClick={() => void confirmDelete()}
            >
              {t("storagePanel.recovery.confirmDelete", "Delete file")}
            </button>
          </>
        ) : (
          <button type="button" className="btn btn-ghost" onClick={onClose}>
            {t("common.close", "Close")}
          </button>
        )
      }
    >
      <p className="text-sm text-base-content/70">
        {t(
          "storagePanel.recovery.intro",
          "Files that {{name}} brought along from an earlier registration. Nothing here is deleted or expired until you decide.",
          { name: repositoryName },
        )}
      </p>

      {pendingDelete ? (
        <div role="alert" className="alert alert-warning alert-soft mt-3 py-2 text-sm">
          {t(
            "storagePanel.recovery.deleteWarning",
            "Delete {{name}} from disk? This cannot be undone. Export it first if you may still need it.",
            { name: recoveryItemFilename(pendingDelete) },
          )}
        </div>
      ) : null}

      {inventory.isPending ? (
        <p className="py-6 text-sm text-base-content/60">{t("common.loading", "Loading…")}</p>
      ) : inventory.isError ? (
        <div role="alert" className="alert alert-error alert-soft mt-3 py-2 text-sm">
          {localizeAPIProblem(
            inventory.error,
            t,
            t("storagePanel.recovery.loadFailed", "The recovery inventory could not be loaded."),
          )}
        </div>
      ) : (
        <>
          <dl className="m-0 mt-3 grid grid-cols-1 gap-x-8 gap-y-1.5 sm:grid-cols-2">
            <div className="flex min-w-0 items-baseline gap-3">
              <dt className="w-28 shrink-0 text-xs text-base-content/55">
                {t("storagePanel.recovery.heldTrash", "Held in Trash")}
              </dt>
              <dd className="m-0 min-w-0 text-xs text-base-content/85">
                {held > 0 ? (
                  <Link className="link" to={`/storage/${repositoryId}/trash`}>
                    {t(
                      "storagePanel.recovery.heldTrashValue",
                      "{{count}} files · decide in Trash",
                      {
                        count: held,
                      },
                    )}
                  </Link>
                ) : (
                  t("storagePanel.recovery.none", "None")
                )}
              </dd>
            </div>
            <div className="flex min-w-0 items-baseline gap-3">
              <dt className="w-28 shrink-0 text-xs text-base-content/55">
                {t("storagePanel.recovery.receipts", "Receipts")}
              </dt>
              <dd className="m-0 min-w-0 text-xs text-base-content/85">
                {receipts.length === 0
                  ? t("storagePanel.recovery.none", "None")
                  : receipts
                      .map((receipt) =>
                        t("storagePanel.recovery.receiptLine", "{{event}} · {{at}}", {
                          event: receipt.event,
                          at: formatDateTime(receipt.recorded_at, i18n.language),
                        }),
                      )
                      .join(" · ")}
              </dd>
            </div>
          </dl>

          {items.length === 0 ? (
            <p className="py-6 text-sm text-base-content/60">
              {t("storagePanel.recovery.empty", "No preserved files need a decision.")}
            </p>
          ) : (
            <ul className="mt-4 divide-y divide-base-300/60">
              {items.map((item) => {
                const actions = item.actions ?? [];
                return (
                  <li key={item.id} className="flex flex-wrap items-center gap-3 py-2">
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm" title={item.path}>
                        {recoveryItemFilename(item)}
                      </p>
                      <p className="text-xs text-base-content/60">
                        {[
                          recoveryClassLabel(t, item.class),
                          recoveryDispositionLabel(t, item.disposition),
                          item.size ? formatBytes(item.size) : null,
                          item.detail || null,
                        ]
                          .filter(Boolean)
                          .join(" · ")}
                      </p>
                    </div>
                    {actions.includes("export") ? (
                      <button
                        type="button"
                        className="btn btn-ghost btn-xs"
                        disabled={exporting === item.id}
                        onClick={() => void runExport(item)}
                        aria-label={t("storagePanel.recovery.exportItem", "Export {{name}}", {
                          name: recoveryItemFilename(item),
                        })}
                      >
                        <Download className="size-4" aria-hidden />
                        {t("storagePanel.recovery.export", "Export")}
                      </button>
                    ) : null}
                    {actions.includes("delete") ? (
                      <button
                        type="button"
                        className="btn btn-ghost btn-xs text-error"
                        disabled={deleteItem.isPending}
                        onClick={() => setPendingDelete(item)}
                        aria-label={t("storagePanel.recovery.deleteItem", "Delete {{name}}", {
                          name: recoveryItemFilename(item),
                        })}
                      >
                        <Trash2 className="size-4" aria-hidden />
                        {t("storagePanel.recovery.delete", "Delete")}
                      </button>
                    ) : null}
                  </li>
                );
              })}
            </ul>
          )}
        </>
      )}
    </Modal>
  );
}
