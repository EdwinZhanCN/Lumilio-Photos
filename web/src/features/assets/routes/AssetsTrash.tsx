import { useCallback } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { useQueryClient } from "@tanstack/react-query";
import { RotateCcw, Trash2 } from "lucide-react";
import ErrorFallback from "@/components/ui/ErrorFallback";
import { AssetsProvider } from "../state/AssetsProvider";
import { AssetsGalleryPage } from "../components/browse/AssetsGalleryPage";
import type { AssetsBulkActionContext, AssetsBulkActionItem } from "@/lib/assets/bulkActions";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useMessage } from "@/features/notifications";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";

const HIDDEN_TRASH_BULK_ACTIONS = [
  "set-rating",
  "set-liked",
  "stack-selected",
  "add-tags",
  "add-to-album",
  "download",
  "delete-assets",
] as const;

const AssetsTrashContent = () => {
  const { t } = useI18n();
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    {
      label: t("collections.sections.utilities", "Utilities"),
      to: "/collections/utilities",
    },
    { label: t("assets.trash.title", "Trash") },
  ]);
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const restoreAssetMutation = $api.useMutation("post", "/api/v1/assets/{id}/restore");

  const invalidateAssetLists = useCallback(async () => {
    await queryClient.invalidateQueries({
      predicate: (query) => {
        const key = query.queryKey;
        if (!Array.isArray(key)) return false;

        const path = key[1];
        return path === "/api/v1/assets/list" || path === "/api/v1/assets/search";
      },
    });
  }, [queryClient]);

  const bulkActions = useCallback(
    (context: AssetsBulkActionContext): AssetsBulkActionItem[] => [
      {
        id: "restore-assets",
        label: t("assets.trash.bulkActions.restore.label", {
          count: context.affectedAssetCount,
        }),
        icon: <RotateCcw size={16} />,
        tone: "info",
        requiresConfirmation: true,
        confirmationTitle: t("assets.trash.bulkActions.restore.confirmTitle"),
        confirmationMessage: t("assets.trash.bulkActions.restore.confirmMessage", {
          count: context.affectedAssetCount,
        }),
        onRun: async ({ selectedAssetIds, affectedAssetCount, clearSelection }) => {
          try {
            await Promise.all(
              selectedAssetIds.map((assetId) =>
                restoreAssetMutation.mutateAsync({
                  params: { path: { id: assetId } },
                  body: {},
                }),
              ),
            );
            await invalidateAssetLists();
            clearSelection();
            showMessage(
              "success",
              t("assets.trash.messages.restoreSuccess", {
                count: affectedAssetCount,
              }),
            );
          } catch (error) {
            console.error("Failed to restore selected assets:", error);
            showMessage("error", t("assets.trash.messages.restoreError"));
            throw error;
          }
        },
      },
    ],
    [invalidateAssetLists, restoreAssetMutation, showMessage, t],
  );

  return (
    <AssetsProvider
      scopeId="assets:trash"
      basePath="/collections/trash"
      defaultSelectionMode="multiple"
    >
      <WorkerProvider>
        <AssetsGalleryPage
          title={t("assets.trash.title")}
          icon={<Trash2 className="h-6 w-6 text-primary" strokeWidth={1.5} />}
          baseFilter={{ is_deleted: true }}
          viewKey="assets:trash"
          bulkActions={bulkActions}
          hiddenBulkActions={HIDDEN_TRASH_BULK_ACTIONS}
        />
      </WorkerProvider>
    </AssetsProvider>
  );
};

const AssetsTrash = () => {
  const { t } = useI18n();

  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallback
          code={500}
          title={t("assets.errorFallback.something_went_wrong")}
          {...props}
        />
      )}
    >
      <AssetsTrashContent />
    </ErrorBoundary>
  );
};

export default AssetsTrash;
