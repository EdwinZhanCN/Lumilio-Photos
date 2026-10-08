import { useCallback, useState } from "react";
import { useParams } from "react-router-dom";
import { RotateCcw, Trash2, ImageOff } from "lucide-react";
import { Modal } from "@/components/ui/Modal";
import { AssetBrowserScope } from "../browse/selection/AssetBrowserScope";
import { AssetBrowser } from "../browse/AssetBrowser";
import type { AssetsBulkActionContext, AssetsBulkActionItem } from "@/lib/assets/bulkActions";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useMessage } from "@/features/notifications";
import { useAssetLifecycle } from "@/lib/assets/useAssetLifecycle";
import {
  missingLabel,
  trashLabel,
  permanentDeleteLabel,
  removeMissingLabel,
  emptyTrashLabel,
} from "@/lib/assets/lifecycleCopy";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import { useI18n } from "@/lib/i18n";

const HIDDEN_ACTIONS = [
  "set-rating",
  "set-liked",
  "stack-selected",
  "add-tags",
  "add-to-album",
  "download",
  "delete-assets",
] as const;

export default function LifecycleAssetsFlow({ state }: { state: "missing" | "trashed" }) {
  const { repositoryId } = useParams<{ repositoryId: string }>();
  const { t } = useI18n();
  const showMessage = useMessage();
  const lifecycle = useAssetLifecycle();
  const [confirmAll, setConfirmAll] = useState(false);
  const isTrash = state === "trashed";
  const title = isTrash ? trashLabel(t) : missingLabel(t);
  const [running, setRunning] = useState(false);
  const basePath = repositoryId
    ? `/storage/${repositoryId}/${isTrash ? "trash" : "missing"}`
    : "/collections/trash";
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    ...(repositoryId
      ? [{ label: t("storagePanel.title", "Storage"), to: "/storage" }]
      : [
          { label: t("sidebar.collections", "Collections"), to: "/collections" },
          { label: t("collections.sections.utilities", "Utilities"), to: "/collections/utilities" },
        ]),
    { label: title },
  ]);
  const irreversibleCopy = isTrash
    ? t(
        "assets.lifecycle.permanentWarning",
        "All trashed copies of the selected assets will be permanently deleted from disk. Assets with no remaining copy and their metadata will be removed. This cannot be undone.",
      )
    : t(
        "assets.lifecycle.missingWarning",
        "Missing entries will be removed. Assets with no remaining copy and their metadata will be removed. No files are deleted. If a file returns later, it is imported as new.",
      );
  const failedCopy = t(
    "assets.lifecycle.actionFailed",
    "The lifecycle action could not be completed.",
  );
  const runAll = async () => {
    if (running) return;
    setRunning(true);
    try {
      if (isTrash)
        await lifecycle.emptyTrash.mutateAsync({
          body: { repository_id: repositoryId, confirm: true },
        });
      else if (repositoryId)
        await lifecycle.removeMissing.mutateAsync({
          body: { repository_id: repositoryId, confirm: true },
        });
      setConfirmAll(false);
      showMessage("success", t("assets.lifecycle.actionComplete", "Action completed."));
    } catch (error) {
      showMessage("error", localizeAPIProblem(error, t, failedCopy));
    } finally {
      setRunning(false);
    }
  };
  const bulkActions = useCallback(
    (context: AssetsBulkActionContext): AssetsBulkActionItem[] => {
      const actions: AssetsBulkActionItem[] = [];
      if (isTrash)
        actions.push({
          id: "restore-assets",
          label: t("assets.trash.bulkActions.restore.label", { count: context.affectedAssetCount }),
          icon: <RotateCcw size={16} />,
          tone: "info",
          requiresConfirmation: true,
          confirmationTitle: t("assets.trash.bulkActions.restore.confirmTitle"),
          confirmationMessage: t("assets.trash.bulkActions.restore.confirmMessage", {
            count: context.affectedAssetCount,
          }),
          onRun: async ({ selectedAssetIds, clearSelection }) => {
            try {
              const result = await lifecycle.restore.mutateAsync({
                body: { asset_ids: selectedAssetIds },
              });
              clearSelection();
              showMessage(
                "success",
                t("assets.trash.messages.restoreSuccess", { count: selectedAssetIds.length }),
              );
              if (result.renamed?.length)
                showMessage(
                  "info",
                  t(
                    "assets.lifecycle.restoredNames",
                    "Occupied paths were preserved. Restored files: {{paths}}",
                    { paths: result.renamed.map((file) => file.restored_path).join(", ") },
                  ),
                );
            } catch (error) {
              showMessage("error", localizeAPIProblem(error, t, failedCopy));
              throw error;
            }
          },
        });
      actions.push({
        id: isTrash ? "delete-permanently" : "remove-missing",
        label: isTrash ? permanentDeleteLabel(t) : removeMissingLabel(t),
        icon: <Trash2 size={16} />,
        tone: "danger",
        requiresConfirmation: true,
        confirmationMessage: irreversibleCopy,
        onRun: async ({ selectedAssetIds, clearSelection }) => {
          try {
            if (isTrash)
              await lifecycle.permanentlyDelete.mutateAsync({
                body: { asset_ids: selectedAssetIds, confirm: true },
              });
            else
              await lifecycle.removeMissing.mutateAsync({
                body: { asset_ids: selectedAssetIds, confirm: true },
              });
            clearSelection();
            showMessage("success", t("assets.lifecycle.actionComplete", "Action completed."));
          } catch (error) {
            showMessage("error", localizeAPIProblem(error, t, failedCopy));
            throw error;
          }
        },
      });
      return actions;
    },
    [failedCopy, irreversibleCopy, isTrash, lifecycle, showMessage, t],
  );
  return (
    <AssetBrowserScope
      scopeId={`assets:${state}:${repositoryId ?? "all"}`}
      basePath={basePath}
      defaultSelectionMode="multiple"
    >
      <WorkerProvider>
        <AssetBrowser
          title={title}
          icon={isTrash ? <Trash2 /> : <ImageOff />}
          constraint={{ lifecycle_state: state, repository_id: repositoryId }}
          viewKey={`assets:${state}:${repositoryId ?? "all"}`}
          bulkActions={bulkActions}
          hiddenBulkActions={HIDDEN_ACTIONS}
          searchEnabled={false}
          hero={
            <div className="flex items-center justify-between gap-4 px-4 py-3">
              <p className="text-sm text-base-content/70">
                {isTrash
                  ? t(
                      "assets.lifecycle.trashHelp",
                      "Restore files before their retention expires, or delete them permanently.",
                    )
                  : t(
                      "assets.lifecycle.missingHelp",
                      "Metadata is kept. Missing assets return automatically when their files are found by a scan.",
                    )}
              </p>
              <button className="btn btn-error btn-sm" onClick={() => setConfirmAll(true)}>
                {isTrash ? emptyTrashLabel(t) : removeMissingLabel(t)}
              </button>
            </div>
          }
        />
        <Modal
          open={confirmAll}
          onClose={() => setConfirmAll(false)}
          dismissable={!running}
          title={isTrash ? emptyTrashLabel(t) : removeMissingLabel(t)}
          size="sm"
          footer={
            <>
              <button
                className="btn btn-ghost"
                disabled={running}
                onClick={() => setConfirmAll(false)}
              >
                {t("common.cancel")}
              </button>
              <button className="btn btn-error" disabled={running} onClick={() => void runAll()}>
                {t("common.confirm", "Confirm")}
              </button>
            </>
          }
        >
          {!repositoryId && (
            <p className="mb-3 font-medium">
              {t("assets.lifecycle.allRepositories", "All Repositories")}
            </p>
          )}
          <p>
            {isTrash
              ? t(
                  "assets.lifecycle.emptyWarning",
                  "All trashed files in this scope will be permanently deleted, including trashed copies of active assets. Other copies are kept. Assets with no remaining entry and their metadata are removed. This cannot be undone.",
                )
              : irreversibleCopy}
          </p>
        </Modal>
      </WorkerProvider>
    </AssetBrowserScope>
  );
}
